package log

import (
	"os"
	"testing"
)

func TestParseRGOutput(t *testing.T) {
	// Create a temporary file with sample rg output
	tmpFile, err := os.CreateTemp("", "rg_output.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// Sample rg output
	sampleOutput := `test.json:1:Failed to get user by id (Line
web/app/Services/IdentityService.php:182:$this->logger->error("Failed to get user by id (Line: " . __LINE__ . ")", [
`
	sources := parseRGOutput(sampleOutput)

	if len(sources) != 2 {
		t.Errorf("Expected 2 sources, got %d", len(sources))
	}

	for _, source := range sources {
		if source.Path == "" || source.Line == 0 {
			t.Errorf("Source mapping has empty fields: %+v", source)
		}
	}
	if sources[0].Path != "test.json" || sources[0].Line != 1 {
		t.Errorf("Expected first source to be test.json:1, got %+v", sources[0])
	}
	if sources[1].Path != "web/app/Services/IdentityService.php" || sources[1].Line != 182 {
		t.Errorf("Expected second source to be web/app/Services/IdentityService.php:182, got %+v", sources[1])
	}
}
