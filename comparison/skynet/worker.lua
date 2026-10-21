local skynet = require "skynet"
local cluster = require "skynet.cluster"

skynet.start(function()
    skynet.dispatch("lua", function(_, _, payload)
        skynet.retpack(payload)
    end)
    cluster.reload({ worker = "127.0.0.1:17002" })
    cluster.register("echo")
    cluster.open("worker")
end)
