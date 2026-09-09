" karya.vim — task/zettel syntax highlighting layered on markdown
" Queries resolved colors from karya binaries via `colors` subcommand.

if exists('g:loaded_karya')
  finish
endif
let g:loaded_karya = 1

" Nearest 256-color index for a hex color
function! s:HexTo256(hex) abort
  let r = str2nr(a:hex[1:2], 16)
  let g = str2nr(a:hex[3:4], 16)
  let b = str2nr(a:hex[5:6], 16)

  " Check grayscale ramp (232-255): 24 shades from #080808 to #EEEEEE
  let gray_avg = (r + g + b) / 3
  let gray_idx = (gray_avg - 8) / 10
  let gray_idx = max([0, min([23, gray_idx])])
  let gray_val = 8 + gray_idx * 10
  let gray_err = abs(r - gray_val) + abs(g - gray_val) + abs(b - gray_val)

  " 6x6x6 color cube (16-231): levels at 0, 95, 135, 175, 215, 255
  let cube_levels = [0, 95, 135, 175, 215, 255]
  let ri = 0 | let gi = 0 | let bi = 0
  let best_r = 0 | let best_g = 0 | let best_b = 0
  for i in range(5)
    if r >= (cube_levels[i] + cube_levels[i+1]) / 2
      let ri = i + 1
    endif
    if g >= (cube_levels[i] + cube_levels[i+1]) / 2
      let gi = i + 1
    endif
    if b >= (cube_levels[i] + cube_levels[i+1]) / 2
      let bi = i + 1
    endif
  endfor
  let best_r = cube_levels[ri]
  let best_g = cube_levels[gi]
  let best_b = cube_levels[bi]
  let cube_err = abs(r - best_r) + abs(g - best_g) + abs(b - best_b)
  let cube_idx = 16 + 36 * ri + 6 * gi + bi

  return gray_err < cube_err ? (232 + gray_idx) : cube_idx
endfunction

function! s:ColorToHighlight(fg, ...) abort
  let bg = a:0 > 0 ? a:1 : ''
  let parts = []
  if a:fg != '' && a:fg =~# '^#'
    call add(parts, 'guifg=' . a:fg)
    if !has('termguicolors') || !&termguicolors
      call add(parts, 'ctermfg=' . s:HexTo256(a:fg))
    endif
  endif
  if bg != '' && bg =~# '^#'
    call add(parts, 'guibg=' . bg)
    if !has('termguicolors') || !&termguicolors
      call add(parts, 'ctermbg=' . s:HexTo256(bg))
    endif
  endif
  return join(parts, ' ')
endfunction

function! s:LoadColors() abort
  silent let json_str = system('todo colors 2>/dev/null')
  if v:shell_error != 0
    return {}
  endif
  try
    return json_decode(json_str)
  catch
    return {}
  endtry
endfunction

function! s:ClearKaryaMatches() abort
  if exists('b:karya_match_ids')
    for id in b:karya_match_ids
      silent! call matchdelete(id)
    endfor
  endif
  let b:karya_match_ids = []
endfunction

function! s:KaryaSyntax() abort
  " Clear syntax state so we re-apply from scratch
  silent! syn clear karyaActive karyaInprogress karyaCompleted karyaSomeday karyaContainer
  silent! syn clear karyaCompletedLine karyaAssignee karyaScheduled karyaDue
  silent! syn clear karyaClock karyaLog karyaJira karyaTag karyaSpecialTag

  let data = s:LoadColors()
  if empty(data)
    return
  endif
  " Cache so RefreshDateHighlights (on every text change) doesn't have to
  " re-invoke `todo colors` — colors don't change while editing.
  let s:karya_data = data

  let keywords_by_cat = {'active': [], 'inprogress': [], 'completed': [], 'someday': [], 'container': []}
  let color_by_cat = {}

  for [kw, info] in items(data.keywords)
    let cat = info.category
    if has_key(keywords_by_cat, cat)
      call add(keywords_by_cat[cat], kw)
    endif
    if !has_key(color_by_cat, cat) && has_key(info, 'fg') && info.fg != ''
      let color_by_cat[cat] = info.fg
    endif
  endfor

  let completed_hi = has_key(color_by_cat, 'completed') ? s:ColorToHighlight(color_by_cat['completed']) : ''

  for category in ['active', 'inprogress', 'completed', 'someday', 'container']
    let kws = keywords_by_cat[category]
    if empty(kws)
      continue
    endif
    let pat = '\%(' . join(kws, '\|') . '\)'
    let group = 'karya' . toupper(category[0]) . category[1:]

    exe 'syn match ' . group . ' /\%(^\|\s\)\zs' . pat . '\ze:/ containedin=ALL'

    let fg = get(color_by_cat, category, '')
    let hi_args = s:ColorToHighlight(fg)
    if category == 'completed'
      exe 'hi ' . group . ' cterm=strikethrough gui=strikethrough ' . hi_args
    elseif hi_args != ''
      exe 'hi ' . group . ' ' . hi_args
    endif
  endfor

  " Completed task lines get strikethrough
  if !empty(keywords_by_cat['completed'])
    let completed_pat = '\%(' . join(keywords_by_cat['completed'], '\|') . '\)'
    exe 'syn match karyaCompletedLine /\%(^\|\s\)\zs' . completed_pat . ':.*$/ containedin=ALL contains=karyaCompleted'
    exe 'hi karyaCompletedLine cterm=strikethrough gui=strikethrough ' . completed_hi
  endif

  " Assignee (always at end of line)
  let assignee_fg = has_key(data.elements, 'assignee') ? get(data.elements.assignee, 'fg', '') : ''
  let assignee_bg = has_key(data.elements, 'assignee') ? get(data.elements.assignee, 'bg', '') : ''
  let assignee_hi = s:ColorToHighlight(assignee_fg, assignee_bg)
  if assignee_hi != ''
    syn match karyaAssignee />> .\+$/ containedin=ALL
    exe 'hi karyaAssignee ' . assignee_hi
  endif

  " Dates (exclude completed lines so they inherit the strikethrough style)
  let date_fg = has_key(data.elements, 'date') ? get(data.elements.date, 'fg', '') : ''
  let date_bg = has_key(data.elements, 'date') ? get(data.elements.date, 'bg', '') : ''
  let date_hi = s:ColorToHighlight(date_fg, date_bg)
  if date_hi != ''
    syn match karyaScheduled /@s:[^ ]\+/ containedin=ALLBUT,karyaCompletedLine
    syn match karyaDue       /@d:[^ ]\+/ containedin=ALLBUT,karyaCompletedLine
    exe 'hi karyaScheduled ' . date_hi
    exe 'hi karyaDue ' . date_hi
  endif
  call s:HighlightDates(data)

  " Clock entries
  syn match karyaClock /CLOCK: .\+/ containedin=ALL
  exe 'hi karyaClock cterm=italic gui=italic' . (completed_hi != '' ? ' ' . completed_hi : '')

  " State transition log entries
  syn match karyaLog /LOG([A-Z_]\+ -> [A-Z_]\+): .\+/ containedin=ALL
  exe 'hi karyaLog cterm=italic gui=italic' . (completed_hi != '' ? ' ' . completed_hi : '')

  " JIRA keys — bold, no underline, so they don't read as clickable links.
  " Muted gruvbox blue (#458588): distinct from link/URL styling and from
  " the red-on-dark-gray used for search highlighting. Only the bracketed
  " form [XXX-999] counts, with space (or line boundary) on both sides —
  " otherwise a ticket ID embedded in a URL (.../browse/XXX-999) matches too.
  syn match karyaJira /\%(^\|\s\)\zs\[[A-Z]\{2,}-\d\+\]\ze\%(\s\|$\)/ containedin=ALL
  exe 'hi karyaJira cterm=bold gui=bold ' . s:ColorToHighlight('#458588')

  " Tags — regular and special (exclude completed lines)
  let tag_fg = has_key(data.elements, 'tag') ? get(data.elements.tag, 'fg', '') : ''
  let tag_bg = has_key(data.elements, 'tag') ? get(data.elements.tag, 'bg', '') : ''
  let tag_hi = s:ColorToHighlight(tag_fg, tag_bg)
  let special_tag_fg = has_key(data.elements, 'special-tag') ? get(data.elements['special-tag'], 'fg', '') : ''
  let special_tag_bg = has_key(data.elements, 'special-tag') ? get(data.elements['special-tag'], 'bg', '') : ''
  let special_tag_hi = s:ColorToHighlight(special_tag_fg, special_tag_bg)

  let special_tags = has_key(data, 'special_tags') ? data.special_tags : []
  if !empty(special_tags)
    let special_pat = '\%(^\|\s\)\zs#\%(' . join(special_tags, '\|') . '\)\>'
    exe 'syn match karyaSpecialTag /' . special_pat . '/ containedin=ALLBUT,karyaCompletedLine'
    if special_tag_hi != ''
      exe 'hi karyaSpecialTag cterm=bold gui=bold ' . special_tag_hi
    else
      hi def link karyaSpecialTag WarningMsg
    endif
  endif

  syn match karyaTag /\%(^\|\s\)\zs#[a-zA-Z0-9_-]\+/ containedin=ALLBUT,karyaCompletedLine,karyaSpecialTag
  if tag_hi != ''
    exe 'hi karyaTag ' . tag_hi
  else
    hi def link karyaTag Identifier
  endif
endfunction

function! s:HighlightDates(data) abort
  " Clear previous overdue/deadline matches — matchaddpos positions don't
  " track edits, so a date hand-edited from past to today/future would
  " otherwise keep its stale highlight until the buffer is reloaded.
  call s:ClearKaryaMatches()

  let today = strftime('%Y-%m-%d')
  let past_fg = has_key(a:data.elements, 'past-date') ? get(a:data.elements['past-date'], 'fg', '') : ''
  let past_bg = has_key(a:data.elements, 'past-date') ? get(a:data.elements['past-date'], 'bg', '') : ''
  let today_fg = has_key(a:data.elements, 'today-date') ? get(a:data.elements['today-date'], 'fg', '') : ''
  let today_bg = has_key(a:data.elements, 'today-date') ? get(a:data.elements['today-date'], 'bg', '') : ''
  let past_hi = s:ColorToHighlight(past_fg, past_bg)
  let today_hi = s:ColorToHighlight(today_fg, today_bg)

  if past_hi != ''
    exe 'hi karyaOverdue cterm=bold gui=bold ' . past_hi
  endif
  if today_hi != ''
    exe 'hi karyaDeadline cterm=bold gui=bold ' . today_hi
  endif

  " Build completed keyword pattern to skip those lines
  let completed_kws = []
  for [kw, info] in items(a:data.keywords)
    if info.category == 'completed'
      call add(completed_kws, kw)
    endif
  endfor
  let completed_pat = empty(completed_kws) ? '' : '\%(^\|\s\)\%(' . join(completed_kws, '\|') . '\):'

  let lnum = 1
  while lnum <= line('$')
    let text = getline(lnum)
    if completed_pat != '' && text =~# completed_pat
      let lnum += 1
      continue
    endif
    let start = 0
    while 1
      let [m, mstart, mend] = matchstrpos(text, '@[sd]:\d\{4}-\d\{2}-\d\{2}[^ ]*', start)
      if mstart == -1
        break
      endif
      let date_str = matchstr(m, '\d\{4}-\d\{2}-\d\{2}')
      if date_str < today
        let mid = matchaddpos('karyaOverdue', [[lnum, mstart + 1, mend - mstart]])
        if mid != -1 | call add(b:karya_match_ids, mid) | endif
      elseif date_str == today
        let mid = matchaddpos('karyaDeadline', [[lnum, mstart + 1, mend - mstart]])
        if mid != -1 | call add(b:karya_match_ids, mid) | endif
      endif
      let start = mend
    endwhile
    let lnum += 1
  endwhile
endfunction

" Re-derive overdue/today date highlighting on every edit, without redoing
" the full syntax rebuild (which shells out to `todo colors`). Handles a
" date hand-edited from a past date to today/future (or vice versa).
function! s:RefreshDateHighlights() abort
  if !exists('s:karya_data') || empty(s:karya_data)
    return
  endif
  call s:HighlightDates(s:karya_data)
endfunction

augroup karya_syntax
  autocmd!
  autocmd FileType markdown call s:KaryaSyntax()
  autocmd BufRead *.md call s:KaryaSyntax()
  autocmd FileChangedShellPost *.md call s:KaryaSyntax()
  autocmd TextChanged,TextChangedI *.md call s:RefreshDateHighlights()
augroup END

" ─── Task Actions ───────────────────────────────────────────────────────────

function! s:ParseTaskLine(line) abort
  if !exists('s:karya_data') || empty(s:karya_data)
    return {}
  endif
  let stripped = substitute(a:line, '^\s*[-*+]\?\s*', '', '')
  for [kw, info] in items(s:karya_data.keywords)
    if stripped =~# '^\V' . kw . '\m:\s'
      let title = substitute(stripped, '^\V' . kw . '\m:\s*', '', '')
      return {'keyword': kw, 'title': title}
    endif
  endfor
  return {}
endfunction

function! s:ProjectFromPath() abort
  let path = expand('%:p')
  " Pattern: <projects-dir>/<project>/notes/<zettel>/README.md
  let m = matchlist(path, '.*/\([^/]\+\)/notes/[^/]\+/[^/]\+$')
  if !empty(m)
    return m[1]
  endif
  " Fallback: inbox file
  if path =~# 'inbox\.md$'
    return 'inbox'
  endif
  " Unstructured: try <projects-dir>/<project>/...
  let m = matchlist(path, '.*/\([^/]\+\)/[^/]\+\.md$')
  if !empty(m)
    return m[1]
  endif
  return '*'
endfunction

function! s:RunTodoCmd(cmd) abort
  let save_pos = getpos('.')
  silent write
  let result = system(a:cmd)
  let result = substitute(result, '\n$', '', '')
  silent! edit!
  call setpos('.', save_pos)
  if v:shell_error == 0
    echohl MoreMsg | echon result | echohl None
  else
    echohl ErrorMsg | echon result | echohl None
  endif
endfunction

function! s:KaryaClockIn() abort
  let parsed = s:ParseTaskLine(getline('.'))
  if empty(parsed)
    echohl ErrorMsg | echon 'Not a task line' | echohl None
    return
  endif
  let project = s:ProjectFromPath()
  let cmd = 'todo clock-in ' . shellescape(project) . ' '
        \ . shellescape(parsed.keyword) . ' ' . shellescape(parsed.title)
  call s:RunTodoCmd(cmd)
endfunction

function! s:KaryaClockOut() abort
  let parsed = s:ParseTaskLine(getline('.'))
  if empty(parsed)
    echohl ErrorMsg | echon 'Not a task line' | echohl None
    return
  endif
  let project = s:ProjectFromPath()
  let cmd = 'todo clock-out ' . shellescape(project) . ' '
        \ . shellescape(parsed.keyword) . ' ' . shellescape(parsed.title)
  call s:RunTodoCmd(cmd)
endfunction

function! s:KaryaTransition() abort
  let parsed = s:ParseTaskLine(getline('.'))
  if empty(parsed)
    echohl ErrorMsg | echon 'Not a task line' | echohl None
    return
  endif
  if !exists('s:karya_data') || empty(s:karya_data)
    echohl ErrorMsg | echon 'No keyword data loaded' | echohl None
    return
  endif

  " Build flat keyword list (all keywords, sorted by category)
  let s:tp_all_keywords = []
  let categories = ['active', 'inprogress', 'completed', 'someday', 'container']
  for cat in categories
    let kws = []
    for [kw, info] in items(s:karya_data.keywords)
      if info.category == cat
        call add(kws, kw)
      endif
    endfor
    call sort(kws)
    call extend(s:tp_all_keywords, kws)
  endfor

  " Store context
  let s:transition_ctx = parsed
  let s:transition_ctx.project = s:ProjectFromPath()
  let s:tp_filter = ''
  let s:tp_cursor = 0

  silent! call prop_type_delete('KaryaPickerGreen')
  call prop_type_add('KaryaPickerGreen', {'highlight': 'String'})
  call s:TransitionOpen()
endfunction

function! s:TransitionFiltered() abort
  if s:tp_filter == ''
    return copy(s:tp_all_keywords)
  endif
  let pat = toupper(s:tp_filter)
  return filter(copy(s:tp_all_keywords), 'stridx(v:val, pat) >= 0')
endfunction

function! s:TransitionRender() abort
  let filtered = s:TransitionFiltered()
  if empty(filtered)
    return [{'text': '  (no matches)', 'props': []}]
  endif
  let lines = []
  for i in range(len(filtered))
    let kw = filtered[i]
    if i == s:tp_cursor
      call add(lines, {'text': '● ' . kw, 'props': []})
    elseif kw == s:transition_ctx.keyword
      let text = '◉ ' . kw
      call add(lines, {'text': text, 'props': [{'col': 1, 'length': len(text), 'type': 'KaryaPickerGreen'}]})
    else
      call add(lines, {'text': '  ' . kw, 'props': []})
    endif
  endfor
  return lines
endfunction

function! s:TransitionOpen() abort
  let lines = s:TransitionRender()
  let title = s:tp_filter == '' ? ' ⌕ type to filter ' : ' ⌕ ' . s:tp_filter . '_ '
  let s:tp_winid = popup_create(lines, #{
        \ filter: function('s:TransitionFilter'),
        \ callback: function('s:TransitionCallback'),
        \ title: title,
        \ highlight: 'Normal',
        \ border: [],
        \ borderchars: ['─', '│', '─', '│', '┌', '┐', '┘', '└'],
        \ borderhighlight: ['Title', 'Comment', 'Comment', 'Comment'],
        \ padding: [0, 1, 0, 1],
        \ minwidth: 20,
        \ maxheight: 15,
        \ line: 'cursor+1',
        \ col: 'cursor',
        \ pos: 'topleft',
        \ cursorline: 0,
        \ mapping: 0,
        \ })
endfunction

function! s:TransitionRefresh() abort
  let filtered = s:TransitionFiltered()
  if s:tp_cursor >= len(filtered)
    let s:tp_cursor = max([0, len(filtered) - 1])
  endif
  let lines = s:TransitionRender()
  call popup_settext(s:tp_winid, lines)
  let title = s:tp_filter == '' ? ' ⌕ type to filter ' : ' ⌕ ' . s:tp_filter . '_ '
  let firstline = s:tp_cursor >= 14 ? s:tp_cursor - 13 : 1
  call popup_setoptions(s:tp_winid, #{title: title, firstline: firstline})
endfunction

function! s:TransitionFilter(winid, key) abort
  if a:key == "\<Esc>"
    call popup_close(a:winid, -1)
    return 1
  elseif a:key == "\<CR>"
    call popup_close(a:winid, 1)
    return 1
  elseif a:key == 'j' || a:key == "\<Down>" || a:key == "\<C-n>"
    let max = len(s:TransitionFiltered()) - 1
    let s:tp_cursor = s:tp_cursor < max ? s:tp_cursor + 1 : 0
    call s:TransitionRefresh()
    return 1
  elseif a:key == 'k' || a:key == "\<Up>" || a:key == "\<C-p>"
    let max = len(s:TransitionFiltered()) - 1
    let s:tp_cursor = s:tp_cursor > 0 ? s:tp_cursor - 1 : max
    call s:TransitionRefresh()
    return 1
  elseif a:key == "\<BS>"
    if s:tp_filter != ''
      let s:tp_filter = s:tp_filter[:-2]
      let s:tp_cursor = 0
      call s:TransitionRefresh()
    endif
    return 1
  elseif a:key =~# '[a-zA-Z_]'
    let s:tp_filter .= a:key
    let s:tp_cursor = 0
    call s:TransitionRefresh()
    return 1
  endif
  return 1
endfunction

function! s:TransitionCallback(winid, result) abort
  if a:result <= 0
    return
  endif
  let filtered = s:TransitionFiltered()
  if empty(filtered) || s:tp_cursor >= len(filtered)
    return
  endif
  let new_kw = filtered[s:tp_cursor]
  if new_kw == s:transition_ctx.keyword
    return
  endif
  let cmd = 'todo transition ' . shellescape(s:transition_ctx.project) . ' '
        \ . shellescape(s:transition_ctx.keyword) . ' '
        \ . shellescape(s:transition_ctx.title) . ' ' . shellescape(new_kw)
  call s:RunTodoCmd(cmd)
endfunction

augroup karya_actions
  autocmd!
  autocmd FileType markdown nnoremap <buffer> <leader>i :call <SID>KaryaClockIn()<CR>
  autocmd FileType markdown nnoremap <buffer> <leader>o :call <SID>KaryaClockOut()<CR>
  autocmd FileType markdown nnoremap <buffer> <leader>t :call <SID>KaryaTransition()<CR>
  autocmd BufRead *.md nnoremap <buffer> <leader>i :call <SID>KaryaClockIn()<CR>
  autocmd BufRead *.md nnoremap <buffer> <leader>o :call <SID>KaryaClockOut()<CR>
  autocmd BufRead *.md nnoremap <buffer> <leader>t :call <SID>KaryaTransition()<CR>
augroup END
