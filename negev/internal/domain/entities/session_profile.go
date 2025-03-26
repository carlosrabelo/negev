package entities

import "regexp"

type SessionProfile struct {
	PromptPattern *regexp.Regexp
	PagerPattern  *regexp.Regexp
	PagerResponse string
}
