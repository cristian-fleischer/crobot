package review

import (
	"fmt"
	"strings"

	"github.com/dizzyc/crobot/internal/platform"
)

// severityIcons maps severity levels to colored circle emoji.
var severityIcons = map[string]string{
	"error":   "\U0001F534", // 🔴
	"warning": "\U0001F7E0", // 🟠
	"info":    "\U0001F535", // 🔵
}

// categoryIcons maps common category labels to representative emoji.
var categoryIcons = map[string]string{
	"security":        "\U0001F512",       // 🔒
	"bug":             "\U0001F41B",       // 🐛
	"performance":     "\u26A1",           // ⚡
	"style":           "\U0001F3A8",       // 🎨
	"maintainability": "\U0001F527",       // 🔧
	"readability":     "\U0001F4D6",       // 📖
	"formatting":      "\U0001F4D0",       // 📐
	"documentation":   "\U0001F4DD",       // 📝
	"docs":            "\U0001F4DD",       // 📝
	"error-handling":  "\U0001F6E1\uFE0F", // 🛡️
	"complexity":      "\U0001F9E9",       // 🧩
}

// defaultCategoryIcon is used when the category has no specific mapping.
const defaultCategoryIcon = "\U0001F4CC" // 📌

// categoryIcon returns the emoji icon for a category, falling back to a
// default pin icon for unknown categories.
func categoryIcon(cat string) string {
	if icon, ok := categoryIcons[strings.ToLower(cat)]; ok {
		return icon
	}
	return defaultCategoryIcon
}

// RenderComment converts a ReviewFinding into the markdown body for an inline
// comment. The output includes a severity icon and badge, category with icon,
// optional severity score, optional criteria, message body, optional code
// suggestion, and a hidden fingerprint for deduplication.
// The botLabel is included as attribution in the comment.
func RenderComment(f platform.ReviewFinding, botLabel string) string {
	var b strings.Builder

	// Severity icon, badge, optional score, and category on the first line.
	icon := severityIcons[f.Severity]
	b.WriteString(fmt.Sprintf("%s **%s**", icon, f.Severity))
	if f.SeverityScore > 0 {
		b.WriteString(fmt.Sprintf(" (%d/10)", f.SeverityScore))
	}
	if f.Category != "" {
		b.WriteString(fmt.Sprintf(" | %s %s", categoryIcon(f.Category), f.Category))
	}
	b.WriteString("\n\n")

	// Criteria line (if any).
	if len(f.Criteria) > 0 {
		b.WriteString(fmt.Sprintf("\U0001F4CB **Criteria:** %s\n\n", strings.Join(f.Criteria, ", ")))
	}

	// Message body.
	b.WriteString(f.Message)
	b.WriteString("\n")

	// Optional suggestion in a fenced code block.
	if f.Suggestion != "" {
		b.WriteString("\n```suggestion\n")
		b.WriteString(f.Suggestion)
		b.WriteString("\n```\n")
	}

	// Hidden fingerprint as a markdown reference-link definition.
	fp := f.Fingerprint
	if fp == "" {
		fp = GenerateFingerprint(&f)
	}
	b.WriteString(fmt.Sprintf("\n[//]: # \"crobot:fp=%s\"", fp))

	// Bot label attribution.
	if botLabel != "" {
		b.WriteString(fmt.Sprintf("\n[//]: # \"crobot:bot=%s\"", botLabel))
	}

	return b.String()
}
