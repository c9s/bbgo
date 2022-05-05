package bbgo

import (
	"os"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/service"
	"github.com/c9s/bbgo/pkg/types"
)

type TestStruct struct {
	Position *types.Position `persistence:"position"`
	Integer  int64           `persistence:"integer"`
	Integer2 int64           `persistence:"integer2"`
}

func (t *TestStruct) InstanceID() string {
	return "test-struct"
}

func preparePersistentServices() []service.PersistenceService {
	mem := service.NewMemoryService()
	jsonDir := &service.JsonPersistenceService{Directory: "testoutput/persistence"}
	pss := []service.PersistenceService{
		mem,
		jsonDir,
	}

	if _, ok := os.LookupEnv("TEST_REDIS"); ok {
		redisP := service.NewRedisPersistenceService(&service.RedisPersistenceConfig{
			Host: "localhost",
			Port: "6379",
			DB:   0,
		})
		pss = append(pss, redisP)
	}

	return pss
}

func Test_callID(t *testing.T) {
	id := callID(&TestStruct{})
	assert.NotEmpty(t, id)
}

func Test_storePersistenceFields(t *testing.T) {
	var pss = preparePersistentServices()

	var a = &TestStruct{
		Integer:  1,
		Integer2: 2,
		Position: types.NewPosition("BTCUSDT", "BTC", "USDT"),
	}

	a.Position.Base = fixedpoint.NewFromFloat(10.0)
	a.Position.AverageCost = fixedpoint.NewFromFloat(3343.0)

	for _, ps := range pss {
		t.Run(reflect.TypeOf(ps).Elem().String(), func(t *testing.T) {
			id := callID(a)
			err := storePersistenceFields(a, id, ps)
			assert.NoError(t, err)

			var i int64
			store := ps.NewStore("test-struct", "integer")
			err = store.Load(&i)
			assert.NoError(t, err)
			assert.Equal(t, int64(1), i)

			var p *types.Position
			store = ps.NewStore("test-struct", "position")
			err = store.Load(&p)
			assert.NoError(t, err)
			assert.Equal(t, fixedpoint.NewFromFloat(10.0), p.Base)
			assert.Equal(t, fixedpoint.NewFromFloat(3343.0), p.AverageCost)

			var b = &TestStruct{}
			err = loadPersistenceFields(b, id, ps)
			assert.NoError(t, err)
			assert.Equal(t, a.Integer, b.Integer)
			assert.Equal(t, a.Integer2, b.Integer2)
			assert.Equal(t, a.Position, b.Position)
		})
	}

}
