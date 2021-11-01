import os

root = os.path.dirname(os.path.abspath(__file__))


def build_linux():
    print("Build Linux")
    linuxenv = f"CGO_ENABLED=0 GOOS=linux GOARCH=amd64"
    last_build_path = os.path.join(root, "qBack")
    build_cmd = f'{linuxenv} go build -ldflags="-s -w" -o {last_build_path}'
    upx_cmd = f'upx --best --lzma {last_build_path}'
    os.system(build_cmd)
    os.system(upx_cmd)


def build_win():
    print("Build Windows")
    winenv = os.environ
    winenv["CGO_ENABLED"] = "0"
    winenv["GOOS"] = "windows"
    winenv["GOARCH"] = "amd64"
    last_build_path = os.path.join(root, "qBack.exe")
    build_cmd = f'go build -ldflags="-s -w" -o {last_build_path}'
    upx_cmd = f'upx --best --lzma {last_build_path}'
    os.system(build_cmd)
    os.system(upx_cmd)


def main():
    build_linux()
    build_win()


if __name__ == "__main__":
    main()
