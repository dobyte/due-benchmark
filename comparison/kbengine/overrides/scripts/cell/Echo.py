import KBEngine


class Echo(KBEngine.Entity):
    def __init__(self):
        KBEngine.Entity.__init__(self)

    def echo(self, token, payload):
        self.base.onEcho(token, payload)
