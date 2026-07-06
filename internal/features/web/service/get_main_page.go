package web_service

import (
	"fmt"
	"os"
	"path"
)

func (s *WebService) GetMainPage() ([]byte, error) {
	htmlFilePath := path.Join(
		os.Getenv("PROJECT_ROOT"),
		"/public/index.html",
	)

	html, err := s.webRepository.GetFile(htmlFilePath)
	if err != nil {
		return nil, fmt.Errorf("get file from repository: %w", err)
	}

	return html, nil
}
