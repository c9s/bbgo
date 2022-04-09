import bbgo_pb2
import bbgo_pb2_grpc


class TestTradingServicer(bbgo_pb2_grpc.TradingServiceServicer):

    def SubmitOrder(self, request, context):
        submit_order = request.submit_order

        order = bbgo_pb2.Order(
            exchange=submit_order.exchange,
            symbol=submit_order.symbol,
            side=submit_order.side,
            quantity=submit_order.quantity,
            price=submit_order.price,
            stop_price=submit_order.stop_price,
            order_type=submit_order.order_type,
            client_order_id=submit_order.client_order_id,
            group_id=submit_order.group_id,
        )

        error = bbgo_pb2.Error(error_code=0, error_message='')

        return bbgo_pb2.SubmitOrderResponse(order=order, error=error)
