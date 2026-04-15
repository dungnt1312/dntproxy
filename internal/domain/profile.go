package domain

// Profile represents a named collection of model aliases that can be activated/deactivated.
// When activated, a profile's aliases are merged into the global modelAliases map.
type Profile struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Aliases     AliasMap `json:"aliases"`          // model name → provider/model
	Combos      []Combo  `json:"combos,omitempty"` // optional embedded combos created with the profile
	CreatedAt   string   `json:"createdAt,omitempty"`
	UpdatedAt   string   `json:"updatedAt,omitempty"`
}
