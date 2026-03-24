package inngest

const truncateBodyMaxLen = 200

// truncateBody truncates a response body string for safe inclusion in error messages.
func truncateBody(body string) string {
	if len(body) <= truncateBodyMaxLen {
		return body
	}
	return body[:truncateBodyMaxLen] + "...(truncated)"
}
