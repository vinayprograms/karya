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
    call add(parts, 'ctermfg=' . s:HexTo256(a:fg))
  endif
  if bg != '' && bg =~# '^#'
    call add(parts, 'guibg=' . bg)
    call add(parts, 'ctermbg=' . s:HexTo256(bg))
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
  " Clear position-based matches from previous load
  call s:ClearKaryaMatches()

  " Clear syntax state so we re-apply from scratch
  silent! syn clear karyaActive karyaInprogress karyaCompleted karyaSomeday
  silent! syn clear karyaCompletedLine karyaAssignee karyaScheduled karyaDue
  silent! syn clear karyaClock karyaLog karyaJira

  let data = s:LoadColors()
  if empty(data)
    return
  endif

  let keywords_by_cat = {'active': [], 'inprogress': [], 'completed': [], 'someday': []}
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

  for category in ['active', 'inprogress', 'completed', 'someday']
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

  " JIRA keys
  syn match karyaJira /\<[A-Z]\{2,}-\d\+\>/ containedin=ALL
  hi def link karyaJira Underlined
endfunction

function! s:HighlightDates(data) abort
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
  let completed_pat = empty(completed_kws) ? '' : '^\s*\%(' . join(completed_kws, '\|') . '\):'

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

augroup karya_syntax
  autocmd!
  autocmd FileType markdown call s:KaryaSyntax()
  autocmd BufRead *.md call s:KaryaSyntax()
  autocmd FileChangedShellPost *.md call s:KaryaSyntax()
augroup END
