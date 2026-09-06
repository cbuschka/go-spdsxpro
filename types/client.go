package types

type Client interface {
	Ping() error
	Close() error
	GetActiveKit() (int, error)
	GetKitList() ([]Kit, error)
	GetSetlistList() ([]Setlist, error)
}

type Kit struct {
	Number   int
	Name     string
	SubTitle string
}

type SetlistStep struct {
	StepNumber int
	KitNumber  int // 1-indexed Kit ID (1..200)
}

type Setlist struct {
	ID    int           // Setlist 1..32
	Name  string        // 12-char Name
	Steps []SetlistStep // Array of kit entries assigned to this setlist
}
