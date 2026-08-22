{
  description = "Ambiente de desenvolvimento e build para o Discord Video Compressor";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; };
        
        # Definição do pacote Go (Produção)
        discord-compress-pkg = pkgs.buildGoModule {
          pname = "discord-compress";
          version = "0.1.0";
          src = ./.;
          
          # CORREÇÃO: null indica ao Nix que este projeto não tem dependências externas (apenas stdlib)
          vendorHash = null; 
          
          # Flags de compilação para reduzir o tamanho do binário (YAGNI/Minimalismo)
          ldflags = [
            "-s"
            "-w"
          ];
          
          meta = with pkgs.lib; {
            description = "Comprime vídeos para o Discord usando FFmpeg";
            license = licenses.mit;
            platforms = platforms.linux ++ platforms.darwin;
          };
        };
      in
      {
        # 1. Ambiente de Desenvolvimento (nix develop)
        devShells.default = pkgs.mkShell {
          name = "discord-compress-dev";

          buildInputs = with pkgs; [
            go
            gopls
            delve
            go-tools
            ffmpeg
            gnumake
          ];

          shellHook = ''
            echo "🚀 Shell de desenvolvimento do Discord Compressor carregado!"
            echo "🐹 Go: $(go version)"
            echo "🎬 FFmpeg: $(ffmpeg -version | head -n 1)"
            echo "💡 Dica: Use 'go run .' para testar ou 'go build' para compilar."
          '';
        };

        # 2. Pacote de Produção (nix build)
        packages.default = discord-compress-pkg;
        packages.discord-compress = discord-compress-pkg;

        # App para executar diretamente: nix run .
        apps.default = {
          type = "app";
          program = "${discord-compress-pkg}/bin/discord-compress";
        };
      }
    );
}
