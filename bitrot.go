package bitrot

// General helper functions
func check(e error) {
	if e != nil {
		panic(e)
	}
}
