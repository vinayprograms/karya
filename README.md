# `karya` - GTD based CLI tool

This is a GTD tool that uses markdown files to capture notes, tasks and other information required to manage tasks. The tool can be used interactively or specific operations can be performed using sub commands.

## Design

This tool assumes the following -

* Each project has a dedicated directory.
* Each project must have a `notes.md` file containing all the information about that project, including open tasks.
* The `notes.md` file can include references to other files in this directory using the standard markdown linking syntax. Files that are specified relative to project directory are also loaded as part of parsing project information.
* If all projects are present under a single root directory, then its path must be specified in `$KARYA_DIR` environment variable. Alternately, you can also have your projects dispersed all across your filesystem. In such a scenario, `$KARYA_MANIFEST` environment variable must point to a manifest file (with or without any extension). Each line of the manifest must contain the absolute path to the project's directory.

## Syntax

`karya` supports the following subcommands - `ls`, `open`, `edit` and `exit`.

### `ls`

This is a context sensitive command that will present information as a list.

* Without additional information `karya ls` will list all projects. This will output the list of projects in `[<directory>] <project-title>` format. Example -
    ```
    [vacation] My 2023 vacation plan
    [groceries] Weekend groceries list
    ```
* When used with project name, for example `karya ls vacation`, it will list all the tasks associated with that project. Tasks that are overdue are highlighted. Tasks that are assigned to someone else are italicized.
    ```
   [1] Decide on budget
        due: 2023-05-25
   [2] Decide dates
        due: 2023-05-26
   [3] Research locations
        due: 2023-05-25     assignee: wife
    ```

### `open`

This sub-command picks a specified project. This can be used to perform specific task commands. For example `karya open vacation finish 2` will mark 2nd task in the vacation list as completed.

#### Task specific commands

The following commands change the state of a specific task. Each task is identified by a number that `karya` assigns it based on where it appears in `notes.md` (or in other markdown files that `notes.md` refers to).

* `finish <task-number>` - Marks the specified task as completed.
* `waitfor <assignee> <task-number>` - Assigns the specified task to the `assignee`
* `schedule <task-number> <date-time>` - Schedules the task to a specific date / time. The date/time format is `YYYY-MM-DDThh:mm:ss`. For example `2023-05-22T15:30` will schedule the task for `3:30PM` on `May 22, 2023`. The tool will automatically assign the local timezone when adding information to `notes.md`. If task's timezone is different from your local timezone, the tool will translate the date & time to your local timezone when listing tasks.
* `defer <task-number>` - Reschedules the task to 7 days from today. Even if the task's scheduled for a specific date, it will be overwritten.
* `due <task-number> <date-time>` - Sets the due date for a specific task. 

The syntax is `karya open <project> <task-command> <task-id>`. For example `karya open vacation finish <task-id>`. You may have to run `karya ls vacation` first to get the task ID.

### `edit`

This sub-command opens a specific project's `notes.md` file in your favourite text editor. The editor is identified using the `$EDITOR` environment variable.

## Autocompletion

When used in non-interative mode, it supports autocompletion in `bash` and `zsh`. Autocompletion is supported for subcommands as well as possible argument options for each sub-command.

## Interactive use

This tool can also be used interactively.
[**TBD**]

At any time, you can use `exit` or `q` to exit interactive mode.

## Directly editing `notes.md`

* When you are capturing project notes, you can edit the markdown files in your favourite editor, even when `karya` is running. This lets you focus on project work while not bothering about task management. You can add tasks, modify their state, etc. and `karya` will promptly work with the updated data when running the next interactive command.
