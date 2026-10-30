package felt

import "github.com/go-playground/validator/v10"

var (
	_ validator.Valuer = Felt{}
	_ validator.Valuer = (*Felt)(nil)
)

// ValidatorValue implements validator.Valuer,
// so every felt type is validated by its hex string.
func (z Typed[K]) ValidatorValue() any {
	return z.String()
}
