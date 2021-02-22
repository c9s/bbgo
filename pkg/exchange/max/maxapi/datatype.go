package max

import (
	"encoding/json"
	"time"
)

type Timestamp time.Time

func (t Timestamp) String() string {
	return time.Time(t).String()
}

func (t *Timestamp) UnmarshalJSON(o []byte) error {
	var timestamp int64
	if err := json.Unmarshal(o, &timestamp); err != nil {
		return err
	}

	*t = Timestamp(time.Unix(timestamp, 0))
	return nil
}
