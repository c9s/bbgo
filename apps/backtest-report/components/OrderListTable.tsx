import {Checkbox, Group, Table} from "@mantine/core";
import React, {useState} from "react";
import {Order} from "../types";

interface OrderListTableProps {
  orders: Order[];
  onClick?: (order: Order) => void;
}

const OrderListTable = (props: OrderListTableProps) => {
  let orders = props.orders;
  const [showCanceledOrders, setShowCanceledOrders] = useState(false);

  if (!showCanceledOrders) {
    orders = orders.filter((order: Order) => {
      return order.status != "CANCELED"
    })
  }

  const rows = orders.map((order: Order) => (
    <tr key={order.order_id} onClick={(e) => {
      props.onClick ? props.onClick(order) : null;
      const nodes = e.currentTarget?.parentNode?.querySelectorAll(".selected")
      nodes?.forEach((node, i) => {
        node.classList.remove("selected")
      })
      e.currentTarget.classList.add("selected")
    }}>
      <td>{order.order_id}</td>
      <td>{order.symbol}</td>
      <td>{order.side}</td>
      <td>{order.order_type}</td>
      <td>{order.price}</td>
      <td>{order.quantity}</td>
      <td>{order.status}</td>
      <td>{order.creation_time.toString()}</td>
    </tr>
  ));

  return <div>
    <Group>
      <Checkbox label="Show Canceled" checked={showCanceledOrders}
                onChange={(event) => setShowCanceledOrders(event.currentTarget.checked)}/>

    </Group>
    <Table highlightOnHover striped>
    <thead>
    <tr>
      <th>Order ID</th>
      <th>Symbol</th>
      <th>Side</th>
      <th>Order Type</th>
      <th>Price</th>
      <th>Quantity</th>
      <th>Status</th>
      <th>Creation Time</th>
    </tr>
    </thead>
    <tbody>{rows}</tbody>
  </Table>
  </div>
}

export default OrderListTable;
