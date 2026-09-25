package main

// exitCode is 1 when any enforced dep failed, else 0. A report never counts.
func exitCode(fs []Finding) int {
	for _, f := range fs {
		if f.Verdict == Fail {
			return 1
		}
	}
	return 0
}

func main() {}
