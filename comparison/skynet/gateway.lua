local skynet = require "skynet"
local socket = require "skynet.socket"
local socketdriver = require "skynet.socketdriver"
local cluster = require "skynet.cluster"

skynet.start(function()
    cluster.reload({ worker = "127.0.0.1:17002" })
    local listener = socket.listen("127.0.0.1", 17001)
    socket.start(listener, function(fd)
        socket.start(fd)
        socketdriver.nodelay(fd)
        while true do
            local header = socket.read(fd, 4)
            if not header then break end
            local size = string.unpack(">I4", header)
            if size < 8 or size > 1048576 then break end
            local payload = socket.read(fd, size)
            if not payload then break end
            -- cluster.call crosses the TCP cluster transport to another OS process.
            local ok, response = pcall(cluster.call, "worker", "@echo", payload)
            if not ok then skynet.error(response); break end
            if not socket.write(fd, string.pack(">s4", response)) then break end
        end
        socket.close(fd)
    end)
end)
