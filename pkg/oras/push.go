package oras

import (
	"context"
	"encoding/json"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/remotes"
	"github.com/docker/distribution"
	"github.com/docker/distribution/manifest"
	"github.com/docker/distribution/manifest/schema2"
	digest "github.com/opencontainers/go-digest"
	specs "github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// Push pushes files to the remote
func Push(ctx context.Context, resolver remotes.Resolver, ref string, provider content.Provider, descriptors []ocispec.Descriptor, opts ...PushOpt) (ocispec.Descriptor, error) {
	if resolver == nil {
		return ocispec.Descriptor{}, ErrResolverUndefined
	}
	if len(descriptors) == 0 {
		return ocispec.Descriptor{}, ErrEmptyDescriptors
	}
	opt := pushOptsDefaults()
	for _, o := range opts {
		if err := o(opt); err != nil {
			return ocispec.Descriptor{}, err
		}
	}
	if opt.validateName != nil {
		for _, desc := range descriptors {
			if err := opt.validateName(desc); err != nil {
				return ocispec.Descriptor{}, err
			}
		}
	}

	pusher, err := resolver.Pusher(ctx, ref)
	if err != nil {
		return ocispec.Descriptor{}, err
	}

	desc, provider, err := pack(provider, descriptors, opt)
	if err != nil {
		return ocispec.Descriptor{}, err
	}

	if err := remotes.PushContent(ctx, pusher, desc, provider, nil, opt.baseHandlers...); err != nil {
		return ocispec.Descriptor{}, err
	}
	return desc, nil
}

func pack(provider content.Provider, descriptors []ocispec.Descriptor, opts *pushOpts) (ocispec.Descriptor, content.Provider, error) {
	store := newHybridStoreFromProvider(provider)

	// Config
	var config ocispec.Descriptor
	if opts.config == nil {
		configBytes := []byte("{}")
		config = ocispec.Descriptor{
			MediaType: ocispec.MediaTypeImageConfig,
			Digest:    digest.FromBytes(configBytes),
			Size:      int64(len(configBytes)),
		}
		store.Set(config, configBytes)
	} else {
		config = *opts.config
	}
	if opts.configAnnotations != nil {
		config.Annotations = opts.configAnnotations
	}
	if opts.configMediaType != "" {
		config.MediaType = opts.configMediaType
	}

	// Manifest
	packManifest := packOCIManifest
	if opts.manifestDocker {
		packManifest = packDockerManifest
	}
	manifestDescriptor, manifestBytes, err := packManifest(config, descriptors, opts)
	if err != nil {
		return ocispec.Descriptor{}, nil, err
	}
	store.Set(manifestDescriptor, manifestBytes)

	return manifestDescriptor, store, nil
}

func packOCIManifest(config ocispec.Descriptor, descriptors []ocispec.Descriptor, opts *pushOpts) (ocispec.Descriptor, []byte, error) {
	manifest := ocispec.Manifest{
		Versioned: specs.Versioned{
			SchemaVersion: 2, // historical value. does not pertain to OCI or docker version
		},
		Config:      config,
		Layers:      descriptors,
		Annotations: opts.manifestAnnotations,
	}
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		return ocispec.Descriptor{}, nil, err
	}
	manifestDescriptor := ocispec.Descriptor{
		MediaType: ocispec.MediaTypeImageManifest,
		Digest:    digest.FromBytes(manifestBytes),
		Size:      int64(len(manifestBytes)),
	}
	return manifestDescriptor, manifestBytes, nil
}

func packDockerManifest(config ocispec.Descriptor, descriptors []ocispec.Descriptor, opts *pushOpts) (ocispec.Descriptor, []byte, error) {
	layers := make([]distribution.Descriptor, len(descriptors))
	for i, desc := range descriptors {
		layers[i] = dockerDescriptorFromOCI(desc)
	}

	manifestV2 := schema2.Manifest{
		Versioned: manifest.Versioned{
			SchemaVersion: 2,
			MediaType:     schema2.MediaTypeManifest,
		},
		Config: dockerDescriptorFromOCI(config),
		Layers: layers,
		// Annotations are dropped as docker does not support it.
	}
	manifestBytes, err := json.Marshal(manifestV2)
	if err != nil {
		return ocispec.Descriptor{}, nil, err
	}
	manifestDescriptor := ocispec.Descriptor{
		MediaType: schema2.MediaTypeManifest,
		Digest:    digest.FromBytes(manifestBytes),
		Size:      int64(len(manifestBytes)),
	}
	return manifestDescriptor, manifestBytes, nil
}

func dockerDescriptorFromOCI(desc ocispec.Descriptor) distribution.Descriptor {
	return distribution.Descriptor{
		MediaType:   desc.MediaType,
		Digest:      desc.Digest,
		Size:        desc.Size,
		URLs:        desc.URLs,
		Annotations: desc.Annotations,
		Platform:    desc.Platform,
	}
}
