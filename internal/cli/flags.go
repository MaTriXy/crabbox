package cli

import "flag"

func flagWasSet(fs *flag.FlagSet, name string) bool {
	seen := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			seen = true
		}
	})
	return seen
}

func FlagWasSet(fs *flag.FlagSet, name string) bool {
	return flagWasSet(fs, name)
}

func extractBoolFlag(args []string, name string) ([]string, bool) {
	want := "--" + name
	out := make([]string, 0, len(args))
	found := false
	for _, arg := range args {
		if arg == want {
			found = true
			continue
		}
		out = append(out, arg)
	}
	return out, found
}
