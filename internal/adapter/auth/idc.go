package auth

// IDC (AWS IAM Identity Center) uses the same device code flow as Builder ID,
// but with a custom startUrl and region provided by the enterprise admin.
//
// The functions in builder-id.go (RegisterClient, StartDeviceAuthorization,
// PollDeviceToken, RefreshTokenSSO) all accept region parameter and work
// for both Builder ID and IDC.
//
// The difference is:
// - Builder ID: startUrl = "https://view.awsapps.com/start", region = "us-east-1"
// - IDC: startUrl = custom (e.g. "https://mycompany.awsapps.com/start"), region = custom
//
// Enterprise IDC accounts may not have a profileArn. In that case, the Q API
// endpoint is used instead of the standard CodeWhisperer endpoint.

// StartIDCDeviceAuth starts device authorization for an IDC enterprise account.
// startURL is the enterprise SSO start URL (e.g. "https://mycompany.awsapps.com/start").
func StartIDCDeviceAuth(startURL, region string) (*RegisterClientResult, *DeviceAuthResult, error) {
	if region == "" {
		region = "us-east-1"
	}

	// Register client
	client, err := RegisterClient(region)
	if err != nil {
		return nil, nil, err
	}

	// Start device authorization with custom startUrl
	deviceAuth, err := StartDeviceAuthorization(client.ClientID, client.ClientSecret, startURL, region)
	if err != nil {
		return client, nil, err
	}

	return client, deviceAuth, nil
}

// StartBuilderIDDeviceAuth starts device authorization for AWS Builder ID.
func StartBuilderIDDeviceAuth() (*RegisterClientResult, *DeviceAuthResult, error) {
	return StartIDCDeviceAuth(KiroConfig.StartURL, "us-east-1")
}
