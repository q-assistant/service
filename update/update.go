package update

const UpdateKindConfig = "config"
const UpdateKindDependency = "dependency"

type UpdateKind string

type Update struct {
	Kind UpdateKind
}

type UpdateFunc func(update *Update)