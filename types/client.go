package types

type PadLayer int

const (
	LayerA PadLayer = 0x40
	LayerB PadLayer = 0x41
)

type Client interface {
	Ping() error
	Close() error
	GetActiveKit() (int, error)
	GetKitList() ([]Kit, error)
	GetSetlistList() ([]Setlist, error)
	GetPadLayerVolume(kitIdx int, padIdx int, layer PadLayer) (int, error)
	SetPadLayerVolume(kitIdx int, padIdx int, layer PadLayer, value int) error
	SetKitName(kitIdx int, name string) error
	SetKitSubTitle(kitIdx int, name string) error
	SetKitClickTempo(kidIdx int, tempoBpm float64) error
	GetKitClickTempo(kidIdx int) (float64, error)
	SetKitPadLinkSend(kitIdx int, padIdx int, tx int) error
	SetKitPadLinkReceive(kitIdx int, padIdx int, rx int) error
	SetKitClickStartRangeFrom(kitIdx int, from int) error
	SetKitClickStartRangeTo(kitIdx int, to int) error
	GetKitClickMode(kitIdx int) (int, error)
	SetKitClickMode(kitIdx int, mode int) error
	GetKitClickSound(kitIdx int) (int, error)
	SetKitClickSound(kitIdx int, mode int) error
	GetKitClickVolume(kitIdx int) (int, error)
	SetKitClickVolume(kitIdx int, volume int) error
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

type ClientOpt func(*ClientConfig)

type ClientConfig struct {
	PortPath string
	Debug    bool
}
