package search

import (
	"crypto/sha256"
	"encoding/binary"
	"math"
)

type EmbedderDescriptor struct {
	Provider        string `json:"provider"`
	RuntimeProvider string `json:"runtime_provider,omitempty"`
	Dimensions      int    `json:"dimensions,omitempty"`
	Semantic        bool   `json:"semantic"`
	Fallback        bool   `json:"fallback,omitempty"`
	Description     string `json:"description,omitempty"`
}

type DescriptorAwareEmbedder interface {
	Descriptor() EmbedderDescriptor
}

type HashEmbedder struct{}

func NewHashEmbedder() *HashEmbedder {
	return &HashEmbedder{}
}

func (e *HashEmbedder) Embed(texts []string, dimensions int) ([][]float32, error) {
	if dimensions <= 0 {
		dimensions = 8
	}
	out := make([][]float32, 0, len(texts))
	for _, text := range texts {
		sum := sha256.Sum256([]byte(text))
		vector := make([]float32, dimensions)
		for i := 0; i < dimensions; i++ {
			offset := (i * 4) % len(sum)
			raw := binary.BigEndian.Uint32(sum[offset : offset+4])
			vector[i] = float32(raw%10000) / 10000
		}
		normalizeVector(vector)
		out = append(out, vector)
	}
	return out, nil
}

type configuredEmbedder struct {
	descriptor EmbedderDescriptor
	delegate   Embedder
}

func NewDevelopmentHashEmbedder(dimensions int) Embedder {
	return configuredEmbedder{
		descriptor: EmbedderDescriptor{
			Provider:        "hash",
			RuntimeProvider: "hash",
			Dimensions:      dimensions,
			Semantic:        false,
			Fallback:        true,
			Description:     "Deterministic development embedder for non-semantic fallback and testing.",
		},
		delegate: NewHashEmbedder(),
	}
}

func NewDisabledEmbedder() Embedder {
	return configuredEmbedder{
		descriptor: EmbedderDescriptor{
			Provider:        "disabled",
			RuntimeProvider: "disabled",
			Semantic:        false,
			Description:     "Semantic embedding is disabled for this deployment.",
		},
		delegate: disabledEmbedder{},
	}
}

func NewFallbackEmbedder(provider string, dimensions int) Embedder {
	return configuredEmbedder{
		descriptor: EmbedderDescriptor{
			Provider:        provider,
			RuntimeProvider: "hash",
			Dimensions:      dimensions,
			Semantic:        false,
			Fallback:        true,
			Description:     "External embedding provider is not configured yet; using deterministic hash fallback.",
		},
		delegate: NewHashEmbedder(),
	}
}

func (e configuredEmbedder) Embed(texts []string, dimensions int) ([][]float32, error) {
	if dimensions <= 0 && e.descriptor.Dimensions > 0 {
		dimensions = e.descriptor.Dimensions
	}
	return e.delegate.Embed(texts, dimensions)
}

func (e configuredEmbedder) Descriptor() EmbedderDescriptor {
	return e.descriptor
}

type disabledEmbedder struct{}

func (disabledEmbedder) Embed(texts []string, dimensions int) ([][]float32, error) {
	out := make([][]float32, 0, len(texts))
	for range texts {
		out = append(out, []float32{})
	}
	return out, nil
}

func normalizeVector(vector []float32) {
	if len(vector) == 0 {
		return
	}
	total := 0.0
	for _, value := range vector {
		total += float64(value * value)
	}
	length := math.Sqrt(total)
	if length == 0 {
		return
	}
	for i, value := range vector {
		vector[i] = float32(float64(value) / length)
	}
}
