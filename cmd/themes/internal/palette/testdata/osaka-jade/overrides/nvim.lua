-- osaka-jade / overrides / nvim.lua
--
-- Sidecar appended by the nvim emitter after the semantic block.
-- Restores the bamboo.nvim palette tuning that CREDITS.md documents was
-- dropped when we adopted the upstream `bamboo.setup({})` verbatim.
--
-- The nvim emitter writes:
--   1. baseline (colorscheme selection, safe fallback)
--   2. semantic (options set from hints.nvim.*)
--   3. hints    (extra keys)
--   4. overrides (this file, verbatim)
--
-- Anything below runs after `require("bamboo").load()`.

local ok, bamboo = pcall(require, "bamboo")
if not ok then return end

-- Colors match the semantic palette in theme.json.
bamboo.setup({
	style = "vulgaris",
	transparent = false,
	term_colors = true,
	code_style = {
		comments   = "italic",
		conditionals = "italic",
		keywords   = "none",
		functions  = "none",
		strings    = "none",
		variables  = "none",
	},
	colors = {
		bright_green = "#63B07A",
		green        = "#549E6A",
		yellow       = "#E5C736",
		orange       = "#DB9F9C",
		red          = "#FF5345",
		grey         = "#627A6C",
		fg           = "#C1C497",
		bg0          = "#111C18",
		bg1          = "#1A2820",
		bg2          = "#23372B",
	},
	highlights = {
		-- Diff colors track palette.semantic.git.*
		DiffAdd    = { fg = "#63B07A" },
		DiffChange = { fg = "#E5C736" },
		DiffDelete = { fg = "#FF5345" },
	},
})
bamboo.load()
