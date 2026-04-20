package permissions

type Choice int

const (
	ChoiceAllow Choice = iota
	ChoiceAllowSession
	ChoiceDeny
)

func Choices(isDangerous bool) []string {
	if isDangerous {
		return []string{"Allow", "Deny"}
	}
	return []string{"Allow", "Allow for this session", "Deny"}
}

func ChoiceAt(cursor int, isDangerous bool) Choice {
	if isDangerous {
		if cursor == 0 {
			return ChoiceAllow
		}
		return ChoiceDeny
	}
	return Choice(cursor)
}
