import os
import platform
import time
import subprocess

root = os.path.dirname(os.path.abspath(__file__))

def get_go_version_info():
    result = subprocess.run(["go", "version"], capture_output=True, text=True)
    parts = result.stdout.strip().split()
    return " ".join(parts[2:])

def update_version(recovery=False):
    file_path = os.path.join(root, "utils", "version.go")
    with open(file_path, "r", encoding="utf-8") as file:
        content = file.read()

    new_date_version = time.strftime("%Y%m%d", time.localtime())
    original_date_version = "COMMIT_DATE"

    new_go_version = get_go_version_info()
    original_go_version = "COMMIT_GOVER"
    if recovery:
        content = content.replace(f'"{new_date_version}"', f'"{original_date_version}"')
        content = content.replace(f'"{new_go_version}"', f'"{original_go_version}"')
    else:
        content = content.replace(f'"{original_date_version}"', f'"{new_date_version}"')
        content = content.replace(f'"{original_go_version}"', f'"{new_go_version}"')

    with open(file_path, "w", encoding="utf-8", newline="\n") as file:
        file.write(content)


def build_linux():
    print("Build Linux")
    linuxenv = "CGO_ENABLED=0 GOOS=linux GOARCH=amd64"
    last_build_path = os.path.join(root, "qback")
    build_cmd = f'{linuxenv} go build -ldflags="-s -w" -o {last_build_path}'
    upx_cmd = f"upx --best --lzma {last_build_path}"
    subprocess.run(build_cmd, shell=True, check=True)
    subprocess.run(upx_cmd, shell=True, check=True)


def build_win():
    print("Build Windows")
    winenv = os.environ.copy()
    winenv["CGO_ENABLED"] = "0"
    winenv["GOOS"] = "windows"
    winenv["GOARCH"] = "amd64"
    last_build_path = os.path.join(root, "qback.exe")
    build_cmd = f'go build -ldflags="-s -w" -o {last_build_path}'
    upx_cmd = f"upx --best --lzma {last_build_path}"
    subprocess.run(build_cmd, shell=True, check=True, env=winenv)
    subprocess.run(upx_cmd, shell=True, check=True)


def main():
    update_version()
    if platform.system() == "Windows":
        build_win()
    elif platform.system() == "Linux":
        build_linux()
    update_version(True)


if __name__ == "__main__":
    main()
