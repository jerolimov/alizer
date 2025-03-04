/*******************************************************************************
 * Copyright (c) 2021 Red Hat, Inc.
 * Distributed under license by Red Hat, Inc. All rights reserved.
 * This program is made available under the terms of the
 * Eclipse Public License v2.0 which accompanies this distribution,
 * and is available at http://www.eclipse.org/legal/epl-v20.html
 *
 * Contributors:
 * Red Hat, Inc.
 ******************************************************************************/
package recognizer

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"

	enricher "github.com/redhat-developer/alizer/go/pkg/apis/enricher"
	"github.com/redhat-developer/alizer/go/pkg/apis/model"
	langfile "github.com/redhat-developer/alizer/go/pkg/utils/langfiles"
	ignore "github.com/sabhiram/go-gitignore"
)

type languageItem struct {
	item   langfile.LanguageItem
	weight int
}

func Analyze(path string) ([]model.Language, error) {
	languagesFile := langfile.Get()
	languagesDetected := make(map[string]languageItem)

	paths, err := GetFilePathsFromRoot(path)
	if err != nil {
		return []model.Language{}, err
	}
	extensionsGrouped := extractExtensions(paths)
	extensionHasProgrammingLanguage := false
	totalProgrammingOccurrences := 0
	for extension := range extensionsGrouped {
		languages := languagesFile.GetLanguagesByExtension(extension)
		if len(languages) == 0 {
			continue
		}
		for _, language := range languages {
			if language.Kind == "programming" {
				var languageFileItem langfile.LanguageItem
				var err error
				if len(language.Group) == 0 {
					languageFileItem = language
				} else {
					languageFileItem, err = languagesFile.GetLanguageByName(language.Group)
					if err != nil {
						continue
					}
				}
				tmpLanguageItem := languageItem{languageFileItem, 0}
				weight := languagesDetected[tmpLanguageItem.item.Name].weight + extensionsGrouped[extension]
				tmpLanguageItem.weight = weight
				languagesDetected[tmpLanguageItem.item.Name] = tmpLanguageItem
				extensionHasProgrammingLanguage = true
			}
		}
		if extensionHasProgrammingLanguage {
			totalProgrammingOccurrences += extensionsGrouped[extension]
			extensionHasProgrammingLanguage = false
		}
	}

	var languagesFound []model.Language
	for name, item := range languagesDetected {
		tmpWeight := float64(item.weight) / float64(totalProgrammingOccurrences)
		tmpWeight = float64(int(tmpWeight*10000)) / 10000
		if tmpWeight > 0.02 {
			tmpLanguage := model.Language{
				Name:           name,
				Aliases:        item.item.Aliases,
				Weight:         tmpWeight * 100,
				Frameworks:     []string{},
				Tools:          []string{},
				CanBeComponent: item.item.Component}
			langEnricher := enricher.GetEnricherByLanguage(name)
			if langEnricher != nil {
				langEnricher.DoEnrichLanguage(&tmpLanguage, &paths)
			}
			languagesFound = append(languagesFound, tmpLanguage)
		}
	}

	sort.SliceStable(languagesFound, func(i, j int) bool {
		return languagesFound[i].Weight > languagesFound[j].Weight
	})

	return languagesFound, nil
}

func extractExtensions(paths []string) map[string]int {
	extensions := make(map[string]int)
	for _, path := range paths {
		extension := filepath.Ext(path)
		if len(extension) == 0 {
			continue
		}
		count := extensions[extension] + 1
		extensions[extension] = count
	}
	return extensions
}

func GetFilePathsFromRoot(root string) ([]string, error) {
	if _, err := os.Stat(root); err != nil {
		return nil, err
	}

	var files []string
	ignoreFile, errorIgnoreFile := getIgnoreFile(root)
	errWalk := filepath.Walk(root,
		func(path string, info os.FileInfo, err error) error {
			if errorIgnoreFile == nil && ignoreFile.MatchesPath(path) {
				if info.IsDir() {
					return filepath.SkipDir
				} else {
					return nil
				}
			}
			if !info.IsDir() && isFileInRoot(root, path) {
				files = append([]string{path}, files...)
			} else {
				files = append(files, path)
			}
			return nil
		})
	return files, errWalk
}

func getIgnoreFile(root string) (*ignore.GitIgnore, error) {
	gitIgnorePath := filepath.Join(root, ".gitignore")
	if _, err := os.Stat(gitIgnorePath); err == nil {
		return ignore.CompileIgnoreFile(gitIgnorePath)
	}
	return nil, errors.New("no git ignore file found")
}

func isFileInRoot(root string, file string) bool {
	dir, _ := filepath.Split(file)
	return strings.EqualFold(filepath.Clean(dir), filepath.Clean(root))
}

func getFilePathsInRoot(root string) ([]string, error) {
	fileInfos, err := ioutil.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, fileInfo := range fileInfos {
		files = append(files, filepath.Join(root, fileInfo.Name()))
	}
	return files, nil
}
