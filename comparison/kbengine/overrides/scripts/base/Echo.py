"""TCP extension driven by KBEngine's poller; RPC uses native EntityCall."""
import socket
import struct
import KBEngine


class Echo(KBEngine.Entity):
    def __init__(self):
        KBEngine.Entity.__init__(self)
        self.clients = {}
        self.next_token = 0
        self.createCellEntityInNewSpace(None)

    def onGetCell(self):
        self.listener = socket.socket()
        self.listener.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
        self.listener.bind(("127.0.0.1", 17001))
        self.listener.listen(1024)
        self.listener.setblocking(False)
        KBEngine.registerReadFileDescriptor(self.listener.fileno(), self.accept)
        print("BENCHMARK_CELL_READY", flush=True)

    def accept(self, _):
        while True:
            try:
                sock, address = self.listener.accept()
            except BlockingIOError:
                return
            sock.setblocking(False)
            sock.setsockopt(socket.IPPROTO_TCP, socket.TCP_NODELAY, 1)
            self.next_token += 1
            token = self.next_token
            self.clients[token] = [sock, bytearray(), bytearray(), False]
            KBEngine.registerReadFileDescriptor(sock.fileno(), lambda fd, t=token: self.read(t))

    def close(self, token):
        client = self.clients.pop(token, None)
        if client:
            sock, incoming, outgoing, writing = client
            KBEngine.deregisterReadFileDescriptor(sock.fileno())
            if writing:
                KBEngine.deregisterWriteFileDescriptor(sock.fileno())
            sock.close()

    def read(self, token):
        client = self.clients.get(token)
        if not client:
            return
        try:
            data = client[0].recv(65536)
            if not data:
                self.close(token)
                return
            client[1].extend(data)
            while len(client[1]) >= 4:
                size = struct.unpack_from(">I", client[1])[0]
                if size < 8 or size > 1048576:
                    self.close(token)
                    return
                if len(client[1]) < 4 + size:
                    return
                payload = bytes(client[1][4:4 + size])
                del client[1][:4 + size]
                self.cell.echo(token, payload)
        except BlockingIOError:
            pass
        except (ConnectionError, OSError):
            self.close(token)

    def onEcho(self, token, payload):
        client = self.clients.get(token)
        if client:
            client[2].extend(struct.pack(">I", len(payload)))
            client[2].extend(payload)
            self.write(token)

    def write(self, token):
        client = self.clients.get(token)
        if not client:
            return
        try:
            while client[2]:
                count = client[0].send(client[2])
                if count == 0:
                    self.close(token)
                    return
                del client[2][:count]
        except BlockingIOError:
            if not client[3]:
                KBEngine.registerWriteFileDescriptor(client[0].fileno(), lambda fd, t=token: self.write(t))
                client[3] = True
            return
        except (ConnectionError, OSError):
            self.close(token)
            return
        if client[3]:
            KBEngine.deregisterWriteFileDescriptor(client[0].fileno())
            client[3] = False
