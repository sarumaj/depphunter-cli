from conan import ConanFile


class App(ConanFile):
    settings = "os", "arch", "compiler", "build_type"
    requires = "openssl/1.1.1t", "zlib/[~1.2]"
    tool_requires = [
        "cmake/3.27.1",  # "ninja/1.11.1" is not required
        "ninja/1.11.1",
    ]

    def requirements(self):
        self.requires("libcurl/8.4.0@", transitive_headers=True)
        self.requires(f"boost/{self.version}")
        # self.requires("poco/1.12.4")
        if self.settings.os == "Windows":
            self.requires('catch2/3.4.0#8c8d1f7a6fb6fd4ba5ab9f0e3c3a4b44')
