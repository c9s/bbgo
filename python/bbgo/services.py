from __future__ import annotations

from typing import Iterator
from typing import List
from typing import Tuple

import bbgo_pb2
import bbgo_pb2_grpc

from .data import ErrorMessage
from .data import KLine
from .data import MarketDataEvent
from .data import Subscription
from .data import UserDataEvent
from .utils import get_insecure_channel


class UserDataService(object):
    stub: bbgo_pb2_grpc.UserDataServiceStub

    def __init__(self, host: str, port: int) -> None:
        self.stub = bbgo_pb2_grpc.UserDataServiceStub(get_insecure_channel(host, port))

    def subscribe(self, session: str) -> Iterator[UserDataEvent]:
        request = bbgo_pb2.UserDataRequest(session)
        response_iter = self.stub.Subscribe(request)

        for response in response_iter:
            yield UserDataEvent.from_pb(response)


class MarketService(object):
    stub: bbgo_pb2_grpc.MarketDataServiceStub

    def __init__(self, host: str, port: int) -> None:
        self.stub = bbgo_pb2_grpc.MarketDataServiceStub(get_insecure_channel(host, port))

    def subscribe(self, subscriptions: List[Subscription]) -> Iterator[MarketDataEvent]:
        request = bbgo_pb2.SubscribeRequest(subscriptions=[s.to_pb() for s in subscriptions])
        response_iter = self.stub.Subscribe(request)

        for response in response_iter:
            yield MarketDataEvent.from_pb(response)

    def query_klines(self,
                     exchange: str,
                     symbol: str,
                     limit: int = 30,
                     interval: str = '1m',
                     start_time: int = None,
                     end_time: int = None) -> Tuple[List[KLine], ErrorMessage]:
        request = bbgo_pb2.QueryKLinesRequest(exchange=exchange,
                                              symbol=symbol,
                                              limit=limit,
                                              interval=interval,
                                              start_time=start_time,
                                              end_time=end_time)

        response = self.stub.QueryKLines(request)

        klines = []
        for kline in response.klines:
            klines.append(KLine.from_pb(kline))

        error = ErrorMessage.from_pb(response.error)

        return klines, error


class TradingService(object):
    stub: bbgo_pb2_grpc.TradingServiceStub

    def __init__(self, host: str, port: int) -> None:
        self.stub = bbgo_pb2_grpc.TradingServiceStub(get_insecure_channel(host, port))

    def submit_order(self,
                     exchange: str,
                     symbol: str,
                     side: bbgo_pb2.Side,
                     quantity: float,
                     order_type: bbgo_pb2.OrderType,
                     price: float = None,
                     stop_price: float = None,
                     client_order_id: float = None,
                     group_id: float = None) -> bbgo_pb2.Order:
        submit_order = bbgo_pb2.SubmitOrder(exchange=exchange,
                                            symbol=symbol,
                                            side=side,
                                            quantity=quantity,
                                            order_type=order_type,
                                            price=price,
                                            stop_price=stop_price,
                                            client_order_id=client_order_id,
                                            group_id=group_id)
        request = bbgo_pb2.SubmitOrderRequest(submit_order=submit_order)
        response = self.stub.SubmitOrder(request)
        return response

    def cancel_order(self, exchange: str, order_id: int, client_order_id: int = None) -> bbgo_pb2.CancelOrderResponse:
        request = bbgo_pb2.CancelOrderRequest(exchange=exchange, id=order_id, client_order_id=client_order_id)
        response = self.stub.CancelOrder(request)
        return response

    def query_order(self, order_id: int = None, client_order_id: int = None) -> bbgo_pb2.QueryOrderResponse:
        request = bbgo_pb2.QueryOrderRequest(id=order_id, client_order_id=client_order_id)
        response = self.stub.QueryOrder(request)
        return response

    def query_orders(self,
                     exchange: str,
                     symbol: str,
                     states: List[str] = None,
                     order_by: str = 'asc',
                     group_id: int = None,
                     pagination: bool = True,
                     page: int = 0,
                     limit: int = 100,
                     offset: int = 0) -> bbgo_pb2.QueryOrdersResponse:
        # set default value to ['wait', 'convert']
        states = states or ['wait', 'convert']
        request = bbgo_pb2.QueryOrdersRequest(exchange=exchange,
                                              symbol=symbol,
                                              states=states,
                                              order_by=order_by,
                                              group_id=group_id,
                                              pagination=pagination,
                                              page=page,
                                              limit=limit,
                                              offset=offset)

        reponse = self.stub.QueryOrders(request)
        return reponse

    def query_trades(self,
                     exchange: str,
                     symbol: str,
                     timestamp: int,
                     order_by: str = 'asc',
                     pagination: bool = True,
                     page: int = 1,
                     limit: int = 100,
                     offset: int = 0) -> bbgo_pb2.QueryTradesResponse:

        request = bbgo_pb2.QueryTradesRequest(exchange=exchange,
                                              symbol=symbol,
                                              timestamp=timestamp,
                                              order_by=order_by,
                                              pagination=pagination,
                                              page=page,
                                              limit=limit,
                                              offset=offset)
        response = self.stub.QueryTrades(request)
        return response
