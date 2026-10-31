package backend

import "context"

// ImagePublication is controller-only. RegistryAuth is explicit private setup
// data, never a browser field, task context, persisted outcome or ambient login.
type ImagePublication struct {
	Identity     PreviewIdentity
	ImageID      string
	Repository   string
	Tag          string
	RegistryAuth string
}
type ImagePublicationResult struct {
	Reference string
	Digest    string
	// Settled means the ORIGINAL push stream ended with a definitive daemon
	// result. Registry availability and application health are separate facts.
	Settled bool
}

// Optional capability; existing backends never silently fall back to a CLI.
// BeforeMutation runs immediately before each original ImageTag/ImagePush.
// An interrupted/unknown push must not be called again, even with the same tag.
type ImagePublisher interface {
	PublishPreviewImage(context.Context, ImagePublication, func(context.Context) error) (ImagePublicationResult, error)
}
