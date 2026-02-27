#!/bin/bash

# This command ensures we start fresh in a new window (-w 0)
# and builds the 3x2 grid step-by-step.

wt.exe -w 0 nt -d . cmd.exe /k "echo Top-Left" \; \
    split-pane -v -d . cmd.exe /k "echo Bottom-Left" \; \
    split-pane -h -s 0.5 -p 0.66 -d . cmd.exe /k "echo Top-Middle" \; \
    split-pane -h -s 0.5 -p 0.66 -d . cmd.exe /k "echo Bottom-Middle" \; \
    split-pane -h -s 0.5 -p 0.5 -d . cmd.exe /k "echo Top-Right" \; \
    split-pane -h -s 0.5 -p 0.5 -d . cmd.exe /k "echo Bottom-Right"