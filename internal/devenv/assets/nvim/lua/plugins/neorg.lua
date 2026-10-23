return {
  {
    "nvim-neorg/neorg",
    lazy = false,
    version = "9.6.4",
    dependencies = {
      "nvim-neorg/tree-sitter-norg",
      "nvim-neorg/tree-sitter-norg-meta",
    },
    opts = {
      load = {
        ["core.defaults"] = {},
        ["core.concealer"] = {},
        ["core.journal"] = {
          config = { workspace = "notes", journal_folder = "journal", strategy = "nested" },
        },
        ["core.dirman"] = {
          config = {
            workspaces = { notes = "~/neorg" },
            default_workspace = "notes",
          },
        },
      },
    },
  },
}
