module.exports = {
  branches: ["main"],
  tagFormat: "v${version}",
  plugins: [
    "@semantic-release/commit-analyzer",
    "@semantic-release/release-notes-generator",
    [
      "@semantic-release/github",
      {
        assets: [
          { path: "dist/*.tar.gz" },
          { path: "dist/*.zip" },
          { path: "dist/checksums.txt" },
          { path: "scripts/install.sh", label: "Unix installer" },
          { path: "scripts/install.ps1", label: "Windows installer" },
        ],
      },
    ],
  ],
};
