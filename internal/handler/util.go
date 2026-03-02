package handler

import "strings"

// extractConclusion extracts structured conclusion from GPT response
// Returns the full response if it's already structured with bullet points or numbered list
func extractConclusion(gptResponse string) string {
	response := strings.TrimSpace(gptResponse)

	// Check if response is already structured (starts with number or bullet point)
	if strings.HasPrefix(response, "1.") || strings.HasPrefix(response, "•") ||
		strings.HasPrefix(response, "-") || strings.HasPrefix(response, "*") {
		return response
	}

	// Try to find conclusion section
	markers := []string{
		"### Заключение\n",
		"### Заключение",
		"## Заключение\n",
		"## Заключение",
		"Заключение:\n",
		"Заключение:",
		"Заключение\n",
	}

	for _, marker := range markers {
		idx := strings.Index(response, marker)
		if idx != -1 {
			conclusion := strings.TrimSpace(response[idx+len(marker):])
			// Remove disclaimer at the end if present
			disclaimers := []string{
				"\n\nИнтерпретация носит информационный характер",
				"\nИнтерпретация носит информационный характер",
				"\n\nThis is for informational purposes",
			}
			for _, disclaimer := range disclaimers {
				if idx := strings.Index(conclusion, disclaimer); idx != -1 {
					conclusion = strings.TrimSpace(conclusion[:idx])
				}
			}
			return strings.TrimSpace(conclusion)
		}
	}

	// If no marker found, return full response (it should already be structured)
	return response
}

