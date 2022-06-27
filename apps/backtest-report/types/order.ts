
export interface Order {
  order_id: number;
  order_type: string;
  side: string;
  symbol: string;
  price: number;
  quantity: number;
  executed_quantity: number;
  status: string;
  update_time: Date;
  creation_time: Date;
  time?: Date;
  tag?: string;
}
