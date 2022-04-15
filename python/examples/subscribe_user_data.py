import grpc
from loguru import logger

import bbgo_pb2
import bbgo_pb2_grpc
from bbgo.data import UserDataEvent


def main():
    host = '127.0.0.1'
    port = 50051
    address = f'{host}:{port}'
    channel = grpc.insecure_channel(address)
    stub = bbgo_pb2_grpc.UserDataServiceStub(channel)

    request = bbgo_pb2.UserDataRequest(session='max')
    response_iter = stub.Subscribe(request)
    for response in response_iter:
        event = UserDataEvent.from_pb(response)
        logger.info(event)


if __name__ == '__main__':
    main()
