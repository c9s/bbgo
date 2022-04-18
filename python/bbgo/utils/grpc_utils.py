import os

import grpc


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


def get_insecure_channel(host: str, port: int) -> grpc.Channel:
    address = f'{host}:{port}'
    return grpc.insecure_channel(address)


def get_insecure_channel_from_env() -> grpc.Channel:
    host = os.environ.get('BBGO_GRPC_HOST') or '127.0.0.1'
    port = os.environ.get('BBGO_GRPC_PORT') or 50051

    address = get_insecure_channel(host, port)

    return grpc.insecure_channel(address)
