package filters

// Filter takes a string and returns a compressed version plus a bool.
// ok=false signals the filter could not usefully compress the input.
type Filter func(string) (string, bool)
