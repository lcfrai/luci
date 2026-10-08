//go:build !unix && !windows

package product

import "os"

func projectOverviewFileIdentity(*os.File) (string, error) { return "", errOverviewQuestions }
