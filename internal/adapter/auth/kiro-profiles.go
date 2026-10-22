package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/dungnt/dntproxy/internal/adapter/shared"
)

// profilesHTTP mirrors the 10s requestTimeout Kiro uses for ListAvailableProfiles.
var profilesHTTP = shared.NewSafeHTTPClient(10 * time.Second)

// Kiro's CodeWhisperer ("q") control-plane regions per partition.
var qRegions = map[string][]string{
	"aws":        {"us-east-1", "eu-central-1"},
	"aws-us-gov": {"us-gov-east-1", "us-gov-west-1"},
}

// qEndpoint builds the CodeWhisperer endpoint for a region. A var so tests can
// point it at a local server.
var qEndpoint = func(region string) string {
	if strings.HasPrefix(region, "us-gov-") {
		return "https://q-fips." + region + ".amazonaws.com"
	}
	return "https://q." + region + ".amazonaws.com"
}

// awsPartition mirrors wVr().
func awsPartition(region string) string {
	switch {
	case strings.HasPrefix(region, "cn-"):
		return "aws-cn"
	case strings.HasPrefix(region, "us-gov-"):
		return "aws-us-gov"
	case strings.HasPrefix(region, "us-isof-"):
		return "aws-iso-f"
	case strings.HasPrefix(region, "us-isob-"):
		return "aws-iso-b"
	case strings.HasPrefix(region, "us-iso-"):
		return "aws-iso"
	}
	return "aws"
}

// KiroProfile is one entry of ListAvailableProfiles.
type KiroProfile struct {
	ARN  string `json:"arn"`
	Name string `json:"profileName"`
}

// profileRegions mirrors getRegionsToQuery: the IdC region's partition when
// known, otherwise the default aws regions.
func profileRegions(idcRegion string) []string {
	if idcRegion == "" {
		return qRegions["aws"]
	}
	return qRegions[awsPartition(idcRegion)]
}

// tokenTypeFor maps the auth method to the TokenType header AWS expects.
func tokenTypeFor(provider string) string {
	switch provider {
	case ProviderBuilderID, ProviderEnterprise, ProviderInternal:
		return "SSO_OIDC"
	case ProviderExternalIdP:
		return "EXTERNAL_IDP"
	}
	return ""
}

func listProfilesInRegion(ctx context.Context, region, accessToken, tokenType string) ([]KiroProfile, error) {
	var out []KiroProfile
	next := ""
	for page := 0; page < 20; page++ {
		body := map[string]interface{}{}
		if next != "" {
			body["nextToken"] = next
		}
		headers := map[string]string{
			"Authorization": "Bearer " + accessToken,
			"Accept":        "application/json",
			"User-Agent":    kiroUserAgent(),
		}
		if tokenType != "" {
			headers["TokenType"] = tokenType
		}
		res, err := doJSON(ctx, "POST", qEndpoint(region)+"/ListAvailableProfiles", body, headers,
			reqOpts{client: profilesHTTP, maxAttempts: 1})
		if err != nil {
			return nil, err
		}
		if res.Status != 200 {
			return nil, fmt.Errorf("ListAvailableProfiles %s: HTTP %d", region, res.Status)
		}
		var r struct {
			Profiles  []KiroProfile `json:"profiles"`
			NextToken string        `json:"nextToken"`
		}
		if err := json.Unmarshal(res.Body, &r); err != nil {
			return nil, err
		}
		for _, p := range r.Profiles {
			if p.ARN != "" && p.Name != "" { // validateProfiles
				out = append(out, p)
			}
		}
		if r.NextToken == "" {
			break
		}
		next = r.NextToken
	}
	return out, nil
}

// ListAvailableProfiles queries every region of the partition in parallel and
// merges the results in region order. Per-region failures are ignored, as in
// Kiro (Promise.allSettled).
func ListAvailableProfiles(ctx context.Context, accessToken, provider, idcRegion string) []KiroProfile {
	regions := profileRegions(idcRegion)
	results := make([][]KiroProfile, len(regions))
	var wg sync.WaitGroup
	for i, region := range regions {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if p, err := listProfilesInRegion(ctx, region, accessToken, tokenTypeFor(provider)); err == nil {
				results[i] = p
			}
		}()
	}
	wg.Wait()
	var all []KiroProfile
	for _, r := range results {
		all = append(all, r...)
	}
	return all
}

// ResolveProfileArn mirrors handleProfiles for Enterprise/Internal IdC. With
// exactly one profile Kiro selects it silently, and so does this. With several
// Kiro prompts the user; dntproxy has nowhere to ask, so it leaves profileArn
// empty — the behaviour IdC connections already had — rather than guess a
// profile. Social tokens already carry their profileArn from the auth service.
//
// Builder ID is deliberately left without a profileArn: Kiro hard-codes a fixed
// ARN for it, but dntproxy has always omitted it for Builder ID and its request
// and quota paths depend on that, so it is not changed here.
func ResolveProfileArn(ctx context.Context, cred *KiroCredential) (string, error) {
	if cred == nil || cred.ProfileArn != "" {
		return "", nil
	}
	switch cred.Provider {
	case ProviderEnterprise, ProviderInternal:
	default:
		return "", nil
	}
	profiles := ListAvailableProfiles(ctx, cred.AccessToken, cred.Provider, cred.Region)
	switch len(profiles) {
	case 0:
		return "", fmt.Errorf("no Kiro profile available for this account")
	case 1:
		return profiles[0].ARN, nil
	}
	return "", fmt.Errorf("%d Kiro profiles available; none selected", len(profiles))
}
