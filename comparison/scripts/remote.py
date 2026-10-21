"""SSH helper; prompts for the password and never stores it in the repository."""
import argparse
import getpass
from pathlib import Path

import paramiko


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--host", default="192.168.2.250")
    parser.add_argument("--user", default="root")
    parser.add_argument("--known-hosts", required=True)
    parser.add_argument("--command")
    parser.add_argument("--interactive", action="store_true")
    parser.add_argument("--upload", nargs=2, metavar=("LOCAL", "REMOTE"))
    parser.add_argument("--download", nargs=2, metavar=("REMOTE", "LOCAL"))
    args = parser.parse_args()
    client = paramiko.SSHClient()
    client.load_host_keys(args.known_hosts)
    client.set_missing_host_key_policy(paramiko.RejectPolicy())
    password = getpass.getpass("SSH password: ")
    client.connect(args.host, username=args.user, password=password,
                   look_for_keys=False, allow_agent=False, timeout=15)
    password = None
    try:
        if args.interactive:
            import json
            print("SSH READY", flush=True)
            while True:
                try:
                    line = input()
                except EOFError:
                    break
                if line == "exit":
                    break
                try:
                    job = json.loads(line)
                    with client.open_sftp() as sftp:
                        if "upload" in job:
                            sftp.put(*job["upload"])
                        if "download" in job:
                            Path(job["download"][1]).parent.mkdir(parents=True, exist_ok=True)
                            sftp.get(*job["download"])
                    if "command" in job:
                        _, stdout, stderr = client.exec_command(job["command"])
                        import time
                        channel = stdout.channel
                        while not channel.exit_status_ready() or channel.recv_ready() or channel.recv_stderr_ready():
                            if channel.recv_ready():
                                print(channel.recv(65536).decode(errors="replace"), end="", flush=True)
                            if channel.recv_stderr_ready():
                                print(channel.recv_stderr(65536).decode(errors="replace"), end="", flush=True)
                            time.sleep(0.05)
                        print("\nSSH EXIT", channel.recv_exit_status(), flush=True)
                    else:
                        print("SSH TRANSFER OK", flush=True)
                except Exception as exc:
                    print(type(exc).__name__, str(exc), flush=True)
            return
        if args.upload:
            with client.open_sftp() as sftp:
                sftp.put(*args.upload)
        if args.download:
            Path(args.download[1]).parent.mkdir(parents=True, exist_ok=True)
            with client.open_sftp() as sftp:
                sftp.get(*args.download)
        if args.command:
            _, stdout, stderr = client.exec_command(args.command)
            channel = stdout.channel
            import time
            while not channel.exit_status_ready() or channel.recv_ready() or channel.recv_stderr_ready():
                if channel.recv_ready():
                    print(channel.recv(65536).decode(errors="replace"), end="", flush=True)
                if channel.recv_stderr_ready():
                    print(channel.recv_stderr(65536).decode(errors="replace"), end="", flush=True)
                time.sleep(0.05)
            raise SystemExit(channel.recv_exit_status())
    finally:
        client.close()


if __name__ == "__main__":
    main()
