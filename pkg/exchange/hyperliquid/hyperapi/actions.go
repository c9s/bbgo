package hyperapi

type OrderWire struct {
	Asset      int           `mapstructure:"a"           msgpack:"a"`
	IsBuy      bool          `mapstructure:"b"           msgpack:"b"`
	LimitPx    string        `mapstructure:"p"           msgpack:"p"`
	Size       string        `mapstructure:"s"           msgpack:"s"`
	ReduceOnly bool          `mapstructure:"r"           msgpack:"r"`
	OrderType  OrderWireType `mapstructure:"t"           msgpack:"t"`
	Cloid      *string       `mapstructure:"c,omitempty" msgpack:"c,omitempty"`
}

type OrderWireType struct {
	Limit   *OrderWireTypeLimit   `mapstructure:"limit,omitempty"   msgpack:"limit,omitempty"`
	Trigger *OrderWireTypeTrigger `mapstructure:"trigger,omitempty" msgpack:"trigger,omitempty"`
}

type OrderWireTypeLimit struct {
	Tif TimeInForce `mapstructure:"tif,string" msgpack:"tif"`
}

type OrderWireTypeTrigger struct {
	IsMarket  bool   `mapstructure:"isMarket"  msgpack:"isMarket"`
	TriggerPx string `mapstructure:"triggerPx" msgpack:"triggerPx"`
	Tpsl      Tpsl   `mapstructure:"tpsl"      msgpack:"tpsl"`
}

type OrderWireBuilderInfo struct {
	Builder string `mapstructure:"b" msgpack:"b"`
	Fee     int    `mapstructure:"f" msgpack:"f"`
}

type OrderAction struct {
	Type     string                `mapstructure:"type"              msgpack:"type"`
	Orders   []OrderWire           `mapstructure:"orders"            msgpack:"orders"`
	Grouping string                `mapstructure:"grouping"          msgpack:"grouping"`
	Builder  *OrderWireBuilderInfo `mapstructure:"builder,omitempty" msgpack:"builder,omitempty"`
}
