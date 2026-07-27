package customize

// CullSettings represents near-duplicate / burst cull preferences.
type CullSettings struct {
	Enabled     bool `json:"enabled" yaml:"Enabled"`
	AutoArchive bool `json:"autoArchive" yaml:"AutoArchive"`
	Window      int  `json:"window" yaml:"Window"`
	SameCamera  bool `json:"sameCamera" yaml:"SameCamera"`
}
