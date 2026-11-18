-- Neovim floating-window keyword picker for karya.vim.
-- Classic Vim uses popup_create() in plugin/karya.vim instead.
local M = {}

local ns = vim.api.nvim_create_namespace('karya_picker')
local MAX_HEIGHT = 15

local function title_for(filter)
  if filter == '' then
    return ' ⌕ type to filter '
  end
  return ' ⌕ ' .. filter .. '_ '
end

function M.open(keywords, current, on_select)
  local state = { filter = '', cursor = 1 }
  local origin_win = vim.api.nvim_get_current_win()

  local function filtered()
    if state.filter == '' then
      return keywords
    end
    local pat = state.filter:upper()
    local out = {}
    for _, kw in ipairs(keywords) do
      if kw:find(pat, 1, true) then
        table.insert(out, kw)
      end
    end
    return out
  end

  local width = 20
  for _, kw in ipairs(keywords) do
    width = math.max(width, vim.fn.strdisplaywidth(kw) + 4)
  end

  local buf = vim.api.nvim_create_buf(false, true)
  vim.bo[buf].bufhidden = 'wipe'

  local win = vim.api.nvim_open_win(buf, true, {
    relative = 'cursor',
    row = 1,
    col = 0,
    width = width,
    height = math.max(1, math.min(MAX_HEIGHT, #keywords)),
    style = 'minimal',
    border = { '┌', '─', '┐', '│', '┘', '─', '└', '│' },
    title = title_for(''),
    title_pos = 'left',
    noautocmd = true,
  })
  vim.wo[win].winhighlight = 'Normal:Normal,NormalFloat:Normal,FloatBorder:Comment,FloatTitle:Title'
  vim.wo[win].cursorline = false
  vim.wo[win].wrap = false

  local closed = false
  local function close()
    if closed then
      return
    end
    closed = true
    if vim.api.nvim_win_is_valid(win) then
      vim.api.nvim_win_close(win, true)
    end
    if vim.api.nvim_win_is_valid(origin_win) then
      vim.api.nvim_set_current_win(origin_win)
    end
  end

  local function render()
    local items = filtered()
    if state.cursor > #items then
      state.cursor = math.max(1, #items)
    end
    local lines = {}
    local hl_rows = {}
    if #items == 0 then
      lines = { '   (no matches)' }
    else
      for i, kw in ipairs(items) do
        if i == state.cursor then
          lines[i] = ' ● ' .. kw
        elseif kw == current then
          lines[i] = ' ◉ ' .. kw
          table.insert(hl_rows, i - 1)
        else
          lines[i] = '   ' .. kw
        end
      end
    end
    vim.bo[buf].modifiable = true
    vim.api.nvim_buf_set_lines(buf, 0, -1, false, lines)
    vim.bo[buf].modifiable = false
    vim.api.nvim_buf_clear_namespace(buf, ns, 0, -1)
    for _, row in ipairs(hl_rows) do
      vim.api.nvim_buf_set_extmark(buf, ns, row, 1, {
        end_col = #lines[row + 1],
        hl_group = 'String',
      })
    end
    vim.api.nvim_win_set_config(win, {
      title = title_for(state.filter),
      title_pos = 'left',
      height = math.max(1, math.min(MAX_HEIGHT, #lines)),
    })
    vim.api.nvim_win_set_cursor(win, { math.max(1, state.cursor), 0 })
  end

  local function move(delta)
    local count = #filtered()
    if count == 0 then
      return
    end
    state.cursor = ((state.cursor - 1 + delta) % count) + 1
    render()
  end

  local function select()
    local items = filtered()
    local choice = items[state.cursor]
    close()
    if choice and choice ~= current then
      vim.schedule(function()
        if type(on_select) == 'string' then
          vim.fn[on_select](choice)
        else
          on_select(choice)
        end
      end)
    end
  end

  local function map(lhs, fn)
    vim.keymap.set('n', lhs, fn, { buffer = buf, nowait = true, silent = true })
  end

  map('<Esc>', close)
  map('<C-c>', close)
  map('<CR>', select)
  for _, key in ipairs({ 'j', '<Down>', '<C-n>' }) do
    map(key, function() move(1) end)
  end
  for _, key in ipairs({ 'k', '<Up>', '<C-p>' }) do
    map(key, function() move(-1) end)
  end
  map('<BS>', function()
    if state.filter ~= '' then
      state.filter = state.filter:sub(1, -2)
      state.cursor = 1
      render()
    end
  end)

  local chars = { '_' }
  for c = string.byte('a'), string.byte('z') do
    if c ~= string.byte('j') and c ~= string.byte('k') then
      table.insert(chars, string.char(c))
    end
  end
  for c = string.byte('A'), string.byte('Z') do
    table.insert(chars, string.char(c))
  end
  for _, ch in ipairs(chars) do
    map(ch, function()
      state.filter = state.filter .. ch
      state.cursor = 1
      render()
    end)
  end

  vim.api.nvim_create_autocmd({ 'WinLeave', 'BufLeave' }, {
    buffer = buf,
    once = true,
    callback = function()
      vim.schedule(close)
    end,
  })

  render()
end

return M
