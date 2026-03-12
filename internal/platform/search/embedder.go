package search

import (
	"crypto/sha256"
	"encoding/binary"
	"math"
)

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
