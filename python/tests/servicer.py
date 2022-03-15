import random
import time

from bbgo import bbgo_pb2
from bbgo import bbgo_pb2_grpc


class TestServicer(bbgo_pb2_grpc.BBGOServicer):

    def Subcribe(self, request, context):
        i = 0
        while True:
            for subscription in request.subscriptions:
                yield bbgo_pb2.SubscribeResponse(
                    channel=subscription.channel,
                    event=bbgo_pb2.Event.UPDATE,
                    exchange=subscription.exchange,
                    symbol=subscription.symbol + f'_{i}',
                )
            i += 1
            time.sleep(random.random())

    def SubcribeUserData(self, request, context):
        i = 0
        while True:
            yield bbgo_pb2.SubscribeResponse(
                channel=bbgo_pb2.Channel.USER,
                event=bbgo_pb2.Event.ORDER_UPDATE,
                exchange='max',
                symbol=f'user_{i}',
                )
            i += 1
            time.sleep(random.random())

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

    def CancelOrder(self, request, context):
        pass

    def QueryOrder(self, request, context):
        pass

    def QueryOrders(self, request, context):
        pass

    def QueryTrades(self, request, context):
        pass

    def QueryKLines(self, request, context):
        pass
