package auth

// IDC (AWS IAM Identity Center) device-code helpers. This is the fallback
// path; the primary Kiro-faithful flow is IdCLogin (kiro-idc-login.go).
//
// Enterprise IDC accounts may not have a profileArn. ResolveProfileArn looks it
// up through ListAvailableProfiles after a successful login.

// StartIDCDeviceAuth starts device authorization for an IDC enterprise account.
// startURL is the enterprise SSO start URL (e.g. "https://mycompany.awsapps.com/start").
func StartIDCDeviceAuth(startURL, region string) (*RegisterClientResult, *DeviceAuthResult, error) {
	if region == "" {
		region = DefaultIdCRegion
	}

	client, err := registerDeviceClient(region)
	if err != nil {
		return nil, nil, err
	}

	deviceAuth, err := StartDeviceAuthorization(client.ClientID, client.ClientSecret, startURL, region)
	if err != nil {
		return client, nil, err
	}

	return client, deviceAuth, nil
}

// StartBuilderIDDeviceAuth starts device authorization for AWS Builder ID.
func StartBuilderIDDeviceAuth() (*RegisterClientResult, *DeviceAuthResult, error) {
	return StartIDCDeviceAuth(KiroConfig.StartURL, DefaultIdCRegion)
}
