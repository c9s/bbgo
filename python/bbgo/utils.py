import os

import grpc

from . import bbgo_pb2_grpc


def read_binary(f):
    with open(f, 'rb') as fp:
        return fp.read()


def get_grpc_cert_file_from_env():
    cert_file = os.environ.get('BBGO_GRPC_CERT_FILE')
    return cert_file


def get_grpc_key_file_from_env():
    key_file = os.environ.get('BBGO_GRPC_KEY_FILE')
    return key_file


def get_credentials_from_env():
    key_file = get_grpc_key_file_from_env()
    private_key = read_binary(key_file)
    cert_file = get_grpc_cert_file_from_env()
    certificate_chain = read_binary(cert_file)

    private_key_certificate_chain_pairs = [(private_key, certificate_chain)]
    server_credentials = grpc.ssl_server_credentials(private_key_certificate_chain_pairs)
    return server_credentials


def create_stub(host, port):
    address = f'{host}:{port}'
    channel = grpc.insecure_channel(address)
    return bbgo_pb2_grpc.BBGOStub(channel)
