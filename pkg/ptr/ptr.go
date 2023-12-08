package ptr

func String(v string) *string {
	return &v
}

func Bool(v bool) *bool {
	return &v
}

func Int32(v int32) *int32 {
	return &v
}
