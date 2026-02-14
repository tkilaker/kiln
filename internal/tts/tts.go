package tts

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// Service handles text-to-speech conversion using a configurable provider.
type Service struct {
	provider Provider
	audioDir string
}

// New creates a new TTS service with the given provider.
func New(provider Provider, audioDir string) (*Service, error) {
	if err := os.MkdirAll(audioDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create audio directory %s: %w", audioDir, err)
	}

	return &Service{
		provider: provider,
		audioDir: audioDir,
	}, nil
}

// GenerateAudio converts article text to an MP3 file.
// Returns the file path and file size.
func (s *Service) GenerateAudio(ctx context.Context, articleID int, text, voice string) (string, int64, error) {
	if voice == "" {
		voice = s.provider.DefaultVoice()
	}

	// Clean up text for TTS
	text = cleanTextForTTS(text)
	if text == "" {
		return "", 0, fmt.Errorf("no text content to convert")
	}

	// Chunk the text based on provider's limit
	maxSize := s.provider.MaxChunkSize()
	chunks := chunkText(text, maxSize)
	log.Printf("TTS [%s]: article %d - %d characters, %d chunk(s), voice=%s",
		s.provider.Name(), articleID, len(text), len(chunks), voice)

	// Generate audio for each chunk
	var audioData bytes.Buffer
	for i, chunk := range chunks {
		log.Printf("TTS [%s]: article %d - generating chunk %d/%d",
			s.provider.Name(), articleID, i+1, len(chunks))

		data, err := s.provider.GenerateChunkAudio(ctx, chunk, voice)
		if err != nil {
			return "", 0, fmt.Errorf("failed to generate audio for chunk %d: %w", i+1, err)
		}
		audioData.Write(data)
	}

	// Write the combined audio to a file
	filename := fmt.Sprintf("%d_%s.mp3", articleID, voice)
	filePath := filepath.Join(s.audioDir, filename)

	if err := os.WriteFile(filePath, audioData.Bytes(), 0644); err != nil {
		return "", 0, fmt.Errorf("failed to write audio file: %w", err)
	}

	fileSize := int64(audioData.Len())
	log.Printf("TTS [%s]: article %d - audio saved to %s (%d bytes)",
		s.provider.Name(), articleID, filePath, fileSize)

	return filePath, fileSize, nil
}

// GetAudioPath returns the expected file path for an article's audio
func (s *Service) GetAudioPath(articleID int, voice string) string {
	if voice == "" {
		voice = s.provider.DefaultVoice()
	}
	filename := fmt.Sprintf("%d_%s.mp3", articleID, voice)
	return filepath.Join(s.audioDir, filename)
}

// AudioDir returns the audio storage directory
func (s *Service) AudioDir() string {
	return s.audioDir
}

// AvailableVoices returns the list of voices from the current provider
func (s *Service) AvailableVoices() []Voice {
	return s.provider.AvailableVoices()
}

// DefaultVoice returns the default voice ID
func (s *Service) DefaultVoice() string {
	return s.provider.DefaultVoice()
}

// ProviderName returns the name of the active TTS provider
func (s *Service) ProviderName() string {
	return s.provider.Name()
}

// cleanTextForTTS prepares text for TTS conversion
func cleanTextForTTS(text string) string {
	// Remove excessive whitespace
	text = strings.TrimSpace(text)

	// Replace multiple newlines with a single pause marker
	lines := strings.Split(text, "\n")
	var cleaned []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			cleaned = append(cleaned, line)
		}
	}

	return strings.Join(cleaned, "\n")
}

// chunkText splits text into chunks that fit within the TTS API's character limit,
// breaking at sentence boundaries when possible
func chunkText(text string, maxSize int) []string {
	if len(text) <= maxSize {
		return []string{text}
	}

	var chunks []string
	remaining := text

	for len(remaining) > 0 {
		if len(remaining) <= maxSize {
			chunks = append(chunks, remaining)
			break
		}

		// Find the best break point within the limit
		chunk := remaining[:maxSize]
		breakPoint := findBreakPoint(chunk)

		chunks = append(chunks, strings.TrimSpace(remaining[:breakPoint]))
		remaining = strings.TrimSpace(remaining[breakPoint:])
	}

	return chunks
}

// findBreakPoint finds the best position to break text, preferring sentence endings
func findBreakPoint(text string) int {
	// Try to break at sentence endings (. ! ?) followed by space
	for i := len(text) - 1; i > len(text)/2; i-- {
		if i+1 < len(text) && (text[i] == '.' || text[i] == '!' || text[i] == '?') && unicode.IsSpace(rune(text[i+1])) {
			return i + 1
		}
	}

	// Fall back to breaking at newline
	for i := len(text) - 1; i > len(text)/2; i-- {
		if text[i] == '\n' {
			return i + 1
		}
	}

	// Fall back to breaking at space
	for i := len(text) - 1; i > len(text)/2; i-- {
		if unicode.IsSpace(rune(text[i])) {
			return i + 1
		}
	}

	// Last resort: break at the limit
	return len(text)
}
