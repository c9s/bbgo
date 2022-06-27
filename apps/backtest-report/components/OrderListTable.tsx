import {Button, Checkbox, Group, Table} from "@mantine/core";
import React, {useState} from "react";
import {Order} from "../types";
import moment from "moment";

interface OrderListTableProps {
  orders: Order[];
  onClick?: (order: Order) => void;
  limit?: number;
}

const OrderListTable = (props: OrderListTableProps) => {
  let orders = props.orders;

  const [showCanceledOrders, setShowCanceledOrders] = useState(false);
  const [limit, setLimit] = useState(props.limit || 100);

  if (!showCanceledOrders) {
    orders = orders.filter((order: Order) => {
      return order.status != "CANCELED"
    })
  }

  if (orders.length > limit) {
    orders = orders.slice(0, limit)
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
      <td>{formatDate(order.creation_time)}</td>
      <td>{order.tag}</td>
    </tr>
  ));

  return <div>
    <Group>
      <Checkbox label="Show Canceled" checked={showCanceledOrders}
                onChange={(event) => setShowCanceledOrders(event.currentTarget.checked)}/>
      <Button onClick={() => {
        setLimit(limit + 500)
      }}>Load More</Button>
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
        <th>Tag</th>
      </tr>
      </thead>
      <tbody>{rows}</tbody>
    </Table>
  </div>
}

const formatDate = (d : Date) : string => {
  return moment(d).format("MMM Do YY hh:mm:ss A Z");
}


export default OrderListTable;
