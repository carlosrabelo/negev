package transport

import (
	"regexp"
	"strings"

	"github.com/carlosrabelo/negev/negev/internal/domain/entities"
)

// defaultSessionProfile matches typical Cisco IOS/NX-OS privileged prompts.
func defaultSessionProfile() entities.SessionProfile {
	return entities.SessionProfile{
		PromptPattern: regexp.MustCompile(`(?m)^[A-Za-z0-9_.-]+(?:-[A-Za-z0-9_.-]+)*[>#]\s*$`),
		PagerPattern:  regexp.MustCompile(`(?m)--More--|---- More ----`),
		PagerResponse: " ",
	}
}

func effectiveProfile(profile entities.SessionProfile) entities.SessionProfile {
	if profile.PromptPattern == nil {
		profile.PromptPattern = defaultSessionProfile().PromptPattern
	}
	if profile.PagerPattern == nil {
		profile.PagerPattern = defaultSessionProfile().PagerPattern
	}
	if profile.PagerResponse == "" {
		profile.PagerResponse = defaultSessionProfile().PagerResponse
	}
	return profile
}

// promptMatched reports whether the buffer ends with a line matching the session prompt.
func promptMatched(profile entities.SessionProfile, buffer string) bool {
	profile = effectiveProfile(profile)
	normalized := strings.ReplaceAll(buffer, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	return profile.PromptPattern.MatchString(normalized)
}

// pagerMatchLoc returns the index range of a pager prompt in the buffer, or nil.
func pagerMatchLoc(profile entities.SessionProfile, buffer string) []int {
	profile = effectiveProfile(profile)
	return profile.PagerPattern.FindStringIndex(buffer)
}

// stripPager removes the matched pager text from the buffer.
func stripPager(buffer string, loc []int) string {
	if loc == nil || loc[0] < 0 || loc[1] > len(buffer) {
		return buffer
	}
	return buffer[:loc[0]] + buffer[loc[1]:]
}

// stripCommandEchoAndPrompt removes the echoed command line and trailing prompt.
func stripCommandEchoAndPrompt(raw string) string {
	normalized := strings.ReplaceAll(raw, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	lines := strings.Split(normalized, "\n")
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) <= 1 {
		return ""
	}
	return strings.Join(lines[1:len(lines)-1], "\n")
}
