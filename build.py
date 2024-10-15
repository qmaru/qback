import os
import platform
import time

root = os.path.dirname(os.path.abspath(__file__))


def update_version(recovery=False):
    file_path = os.path.join(root, "cmd", "root.go")
    with open(file_path, "r", encoding="utf-8") as file:
        content = file.read()

    new_version = time.strftime("%Y%m%d", time.localtime())
    original_version = "VERSION"
    if recovery:
        updated_content = content.replace(f'"{new_version}"', original_version)
    else:
        updated_content = content.replace(original_version, f'"{new_version}"')

    with open(file_path, "w", encoding="utf-8") as file:
        file.write(updated_content)


def build_linux():
    print("Build Linux")
    linuxenv = "CGO_ENABLED=0 GOOS=linux GOARCH=amd64"
    last_build_path = os.path.join(root, "qBack")
    build_cmd = f'{linuxenv} go build -ldflags="-s -w" -o {last_build_path}'
    upx_cmd = f"upx --best --lzma {last_build_path}"
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
    upx_cmd = f"upx --best --lzma {last_build_path}"
    os.system(build_cmd)
    os.system(upx_cmd)


def main():
    update_version()
    if platform.system() == "Windows":
        build_win()
    elif platform.system() == "Linux":
        build_linux()
    update_version(True)


if __name__ == "__main__":
    main()
