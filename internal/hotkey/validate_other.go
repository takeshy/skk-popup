//go:build !darwin

package hotkey

func ValidateAccelerator(s string) error {
	_, _, err := ParseAccelerator(s)
	return err
}
