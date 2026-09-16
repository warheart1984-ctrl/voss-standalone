package main

// The Voss runtime runs either the REST router (default) or one of the
// deployed support services selected by VOSS_MODE. See modes.go.
func main() {
	RunService()
}
