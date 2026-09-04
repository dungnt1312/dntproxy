package mcp

import "errors"

var (
	errImageURLRequired = errors.New("image_url is required (https URL or data: URI)")
	errVideoURLRequired = errors.New("video_url is required (https URL)")
)
