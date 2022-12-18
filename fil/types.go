package fil

import (
	"fmt"
	"strconv"

	filaddr "github.com/filecoin-project/go-address"
	"golang.org/x/xerrors"
)

type ActorID uint64 //nolint:revive

func (a ActorID) String() string { return fmt.Sprintf("f0%d", a) }
func (a ActorID) AsFilAddr() filaddr.Address { //nolint:revive
	r, _ := filaddr.NewIDAddress(uint64(a))
	return r
}

func ParseActorString(s string) (ActorID, error) { //nolint:revive
	if len(s) < 3 || (s[:2] != "f0") {
		return 0, xerrors.Errorf("input '%s' does not have expected prefix", s)
	}

	val, err := strconv.ParseUint(s[2:], 10, 64)
	if err != nil {
		return 0, xerrors.Errorf("unable to parse value of input '%s': %w", s, err)
	}

	return ActorID(val), nil
}

func MustParseActorString(s string) ActorID { //nolint:revive
	a, err := ParseActorString(s)
	if err != nil {
		panic(fmt.Sprintf("unexpected error parsing '%s': %s", s, err))
	}
	return a
}
