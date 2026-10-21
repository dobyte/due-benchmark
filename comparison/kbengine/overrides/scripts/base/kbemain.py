import KBEngine


def onBaseAppReady(isBootstrap):
    if isBootstrap:
        KBEngine.createEntityLocally("Echo", {})


def onReadyForLogin(isBootstrap):
    return 1.0


def onReadyForShutDown():
    return True
