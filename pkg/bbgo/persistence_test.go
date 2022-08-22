package bbgo

import (
	"os"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/dynamic"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/service"
	"github.com/c9s/bbgo/pkg/types"
)

type TestStructWithoutInstanceID struct {
	Symbol string
}

func (s *TestStructWithoutInstanceID) ID() string {
	return "test-struct-no-instance-id"
}

type TestStruct struct {
	*Environment

	Position *types.Position `persistence:"position"`
	Integer  int64           `persistence:"integer"`
	Integer2 int64           `persistence:"integer2"`
	Float    int64           `persistence:"float"`
	String   string          `persistence:"string"`
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

func Test_CallID(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		id := dynamic.CallID(&TestStruct{})
		assert.NotEmpty(t, id)
		assert.Equal(t, "test-struct", id)
	})

	t.Run("fallback", func(t *testing.T) {
		id := dynamic.CallID(&TestStructWithoutInstanceID{Symbol: "BTCUSDT"})
		assert.Equal(t, "test-struct-no-instance-id:BTCUSDT", id)
	})
}

func Test_loadPersistenceFields(t *testing.T) {
	var pss = preparePersistentServices()

	for _, ps := range pss {
		psName := reflect.TypeOf(ps).Elem().String()
		t.Run(psName+"/empty", func(t *testing.T) {
			b := &TestStruct{}
			err := loadPersistenceFields(b, "test-empty", ps)
			assert.NoError(t, err)
		})

		t.Run(psName+"/nil", func(t *testing.T) {
			var b *TestStruct = nil
			err := loadPersistenceFields(b, "test-nil", ps)
			assert.Equal(t, dynamic.ErrCanNotIterateNilPointer, err)
		})

		t.Run(psName+"/pointer-field", func(t *testing.T) {
			var a = &TestStruct{
				Position: types.NewPosition("BTCUSDT", "BTC", "USDT"),
			}
			a.Position.Base = fixedpoint.NewFromFloat(10.0)
			a.Position.AverageCost = fixedpoint.NewFromFloat(3343.0)
			err := storePersistenceFields(a, "pointer-field-test", ps)
			assert.NoError(t, err)

			b := &TestStruct{}
			err = loadPersistenceFields(b, "pointer-field-test", ps)
			assert.NoError(t, err)

			assert.Equal(t, "10", a.Position.Base.String())
			assert.Equal(t, "3343", a.Position.AverageCost.String())
		})
	}
}

func Test_storePersistenceFields(t *testing.T) {
	var pss = preparePersistentServices()

	var a = &TestStruct{
		Integer:  1,
		Integer2: 2,
		Float:    3.0,
		String:   "foobar",
		Position: types.NewPosition("BTCUSDT", "BTC", "USDT"),
	}

	a.Position.Base = fixedpoint.NewFromFloat(10.0)
	a.Position.AverageCost = fixedpoint.NewFromFloat(3343.0)

	for _, ps := range pss {
		psName := reflect.TypeOf(ps).Elem().String()
		t.Run("all/"+psName, func(t *testing.T) {
			id := dynamic.CallID(a)
			err := storePersistenceFields(a, id, ps)
			assert.NoError(t, err)

			var i int64
			store := ps.NewStore("state", "test-struct", "integer")
			err = store.Load(&i)
			assert.NoError(t, err)
			assert.Equal(t, int64(1), i)

			var p *types.Position
			store = ps.NewStore("state", "test-struct", "position")
			err = store.Load(&p)
			assert.NoError(t, err)
			assert.Equal(t, fixedpoint.NewFromFloat(10.0), p.Base)
			assert.Equal(t, fixedpoint.NewFromFloat(3343.0), p.AverageCost)

			var b = &TestStruct{}
			err = loadPersistenceFields(b, id, ps)
			assert.NoError(t, err)
			assert.Equal(t, a.Integer, b.Integer)
			assert.Equal(t, a.Integer2, b.Integer2)
			assert.Equal(t, a.Float, b.Float)
			assert.Equal(t, a.String, b.String)
			assert.Equal(t, a.Position, b.Position)
		})
	}

}
