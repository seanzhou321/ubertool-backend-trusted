#!/usr/bin/env bash
# load-context.sh - List and load pi agent contexts
# Usage: ./load-context.sh [filter|--list|--latest]

set -euo pipefail

CONTEXTS_ROOT="${PWD}/.pi/context"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Find all context files
find_contexts() {
    # Look for both JSONL conversation logs and markdown summaries
    find "$CONTEXTS_ROOT" \( -name "*.jsonl" -o -name "*.md" \) -type f 2>/dev/null | sort -r
}

# Extract context info from first line of JSONL file, or from filename for markdown
get_context_info() {
    local file="$1"
    local first_line
    first_line=$(head -1 "$file" 2>/dev/null || echo "")
    
    # Check if it's a markdown file (ends with .md)
    if [[ "$file" == *.md ]]; then
        local filename
        filename=$(basename "$file")
        # Format: N-context-summary.md or N-context-summary-2026-08-07.md
        local prefix="${filename%%-context-summary*}"
        local rest="${filename#${prefix}-context-summary}"
        local timestamp="${rest%.md}"
        if [[ -z "$timestamp" || "$timestamp" == "$filename" ]]; then
            timestamp="unknown"
        else
            # Clean up leading dash
            timestamp="${timestamp#-}"
        fi
        echo "summary-${prefix}|$timestamp|unknown|$file"
        return 0
    fi
    
    if [[ -z "$first_line" ]]; then
        return 1
    fi
    
    # Parse JSON using jq if available, otherwise use basic parsing
    if command -v jq &> /dev/null; then
        local id timestamp cwd
        id=$(echo "$first_line" | jq -r '.id // "unknown"')
        timestamp=$(echo "$first_line" | jq -r '.timestamp // "unknown"')
        cwd=$(echo "$first_line" | jq -r '.cwd // "unknown"')
        echo "$id|$timestamp|$cwd|$file"
    else
        # Fallback: extract from filename
        local filename
        filename=$(basename "$file")
        # Format: 2026-08-07T03-09-53-073Z_019fda32-d831-7533-87ba-639cfc0bb816.jsonl
        local timestamp_part="${filename%%_*}"
        local id_part="${filename#*_}"
        id_part="${id_part%.jsonl}"
        # Convert timestamp format back
        local timestamp="${timestamp_part//-/:}"
        timestamp="${timestamp//T/ }"
        echo "$id_part|$timestamp|unknown|$file"
    fi
}

# List all contexts with formatting
list_contexts() {
    local filter="${1:-}"
    local contexts=()
    
    while IFS= read -r file; do
        [[ -f "$file" ]] || continue
        local info
        info=$(get_context_info "$file") || continue
        
        IFS='|' read -r id timestamp cwd filepath <<< "$info"
        
        # Apply filter if provided
        if [[ -n "$filter" ]]; then
            if [[ "$id" != "$filter"* && "$timestamp" != "$filter"* ]]; then
                continue
            fi
        fi
        
        contexts+=("$info")
    done < <(find_contexts)
    
    if [[ ${#contexts[@]} -eq 0 ]]; then
        echo -e "${YELLOW}No contexts found${NC}" >&2
        return 1
    fi
    
    echo -e "${CYAN}Available Contexts:${NC}" >&2
    echo >&2
    
    local i=1
    for info in "${contexts[@]}"; do
        IFS='|' read -r id timestamp cwd filepath <<< "$info"
        # Format timestamp for display
        local display_time="$timestamp"
        if [[ "$timestamp" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}T ]]; then
            display_time="${timestamp/T/ }"
            display_time="${display_time/Z/}"
        fi
        printf "  ${GREEN}%2d${NC}  ${YELLOW}%s${NC}  ${BLUE}%s${NC}  %s\n" "$i" "$display_time" "$id" "$cwd" >&2
        ((i++))
    done
    
    # Output filepaths for selection (one per line)
    for info in "${contexts[@]}"; do
        IFS='|' read -r id timestamp cwd filepath <<< "$info"
        echo "$filepath"
    done
}

# Display a context in readable format
display_context() {
    local file="$1"

    if [[ ! -f "$file" ]]; then
        echo -e "${RED}Context file not found: $file${NC}" >&2
        return 1
    fi
    
    # Check if markdown file
    if [[ "$file" == *.md ]]; then
        echo -e "${CYAN}=== Context Summary (Markdown) ===${NC}"
        echo -e "File: $file"
        echo
        cat "$file"
        return 0
    fi
    
    local first_line
    first_line=$(head -1 "$file")
    
    local id timestamp cwd model provider
    if command -v jq &> /dev/null; then
        id=$(echo "$first_line" | jq -r '.id // "unknown"')
        timestamp=$(echo "$first_line" | jq -r '.timestamp // "unknown"')
        cwd=$(echo "$first_line" | jq -r '.cwd // "unknown"')
    else
        id="unknown"
        timestamp="unknown"
        cwd="unknown"
    fi
    
    echo -e "${CYAN}=== Context: $id ===${NC}"
    echo -e "Date: $timestamp"
    echo -e "CWD: $cwd"
    echo
    
    # Process each line
    local line_num=0
    while IFS= read -r line; do
        ((line_num+=1))
        [[ -z "$line" ]] && continue
        
        if ! command -v jq &> /dev/null; then
            echo "  [Line $line_num] $line"
            continue
        fi
        
        local type
        type=$(echo "$line" | jq -r '.type // "unknown"')
        
        case "$type" in
            "session"|"context")
                # Already displayed header
                ;;
            "model_change")
                model=$(echo "$line" | jq -r '.modelId // "unknown"')
                provider=$(echo "$line" | jq -r '.provider // "unknown"')
                echo -e "${BLUE}Model: $model ($provider)${NC}"
                ;;
            "thinking_level_change")
                local level
                level=$(echo "$line" | jq -r '.thinkingLevel // "unknown"')
                echo -e "${BLUE}Thinking Level: $level${NC}"
                ;;
            "message")
                local role timestamp_ms content
                role=$(echo "$line" | jq -r '.message.role // "unknown"')
                timestamp_ms=$(echo "$line" | jq -r '.timestamp // 0')
                # Convert timestamp to readable (handles both Unix ms and ISO string)
                local msg_time
                if [[ "$timestamp_ms" =~ ^[0-9]+$ ]] && [[ "$timestamp_ms" -gt 0 ]]; then
                    # Unix milliseconds
                    msg_time=$(node -e "console.log(new Date($timestamp_ms).toLocaleTimeString())" 2>/dev/null || echo "")
                elif [[ "$timestamp_ms" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}T ]]; then
                    # ISO string
                    msg_time=$(node -e "console.log(new Date('$timestamp_ms').toLocaleTimeString())" 2>/dev/null || echo "")
                else
                    msg_time=""
                fi
                
                if [[ "$role" == "user" ]]; then
                    local text
                    text=$(echo "$line" | jq -r '.message.content[0].text // ""' 2>/dev/null || echo "")
                    echo -e "${GREEN}--- User${msg_time:+ ($msg_time)} ---${NC}"
                    echo "$text" | sed 's/^/  /'
                    echo
                elif [[ "$role" == "assistant" ]]; then
                    echo -e "${YELLOW}--- Assistant${msg_time:+ ($msg_time)} ---${NC}"
                    
                    # Process content blocks
                    local content_blocks
                    content_blocks=$(echo "$line" | jq -c '.message.content[]' 2>/dev/null || echo "")
                    
                    while IFS= read -r block; do
                        [[ -z "$block" ]] && continue
                        local block_type
                        block_type=$(echo "$block" | jq -r '.type // ""')
                        
                        case "$block_type" in
                            "thinking")
                                local thinking
                                thinking=$(echo "$block" | jq -r '.thinking // ""')
                                echo -e "  ${CYAN}[Thinking]${NC} $thinking" | sed 's/^/  /'
                                ;;
                            "toolCall")
                                local tool_name tool_args
                                tool_name=$(echo "$block" | jq -r '.name // ""')
                                tool_args=$(echo "$block" | jq -r '.arguments // "{}"' | jq -c '.')
                                echo -e "  ${BLUE}[Tool: $tool_name]${NC} $tool_args"
                                ;;
                            "text")
                                local text
                                text=$(echo "$block" | jq -r '.text // ""')
                                [[ -n "$text" ]] && echo "$text" | sed 's/^/  /'
                                ;;
                            *)
                                echo "  [Block: $block_type] $block" | sed 's/^/  /'
                                ;;
                        esac
                    done <<< "$content_blocks"
                    echo
                fi
                ;;
            "tool_result"|"toolResult")
                # Some context formats may have tool results as separate entries
                local tool_name result
                tool_name=$(echo "$line" | jq -r '.name // .tool // "unknown"')
                result=$(echo "$line" | jq -r '.result // .output // ""' | head -c 500)
                echo -e "  ${GREEN}[Tool Result: $tool_name]${NC} $result..."
                ;;
            *)
                # Unknown type, show raw
                echo "  [$type] $line" | sed 's/^/  /'
                ;;
        esac
    done < "$file"
}

# Interactive selection
select_context() {
    local filter="${1:-}"
    local contexts
    contexts=$(list_contexts "$filter")
    local exit_code=$?
    
    if [[ $exit_code -ne 0 ]]; then
        return 1
    fi
    
    # Read filepaths into array
    local filepaths=()
    while IFS= read -r line; do
        filepaths+=("$line")
    done <<< "$contexts"
    
    if [[ ${#filepaths[@]} -eq 1 ]]; then
        echo "${filepaths[0]}"
        return 0
    fi

    echo -e "${CYAN}Enter context number (1-${#filepaths[@]}) or 'q' to quit:${NC}" >&2
    read -r choice
    
    if [[ "$choice" == "q" || "$choice" == "Q" ]]; then
        return 1
    fi
    
    if [[ "$choice" =~ ^[0-9]+$ ]] && [[ "$choice" -ge 1 ]] && [[ "$choice" -le ${#filepaths[@]} ]]; then
        echo "${filepaths[$((choice - 1))]}"
        return 0
    fi
    
    echo -e "${RED}Invalid selection${NC}" >&2
    return 1
}

# Main
main() {
    local arg="${1:-}"
    
    case "$arg" in
        --list)
            list_contexts ""
            ;;
        --latest)
            local latest
            # Read first line without head to avoid SIGPIPE
            while IFS= read -r latest; do
                break
            done < <(bash -c "source \"${BASH_SOURCE[0]}\" 2>/dev/null; list_contexts \"\" 2>/dev/null")
            if [[ -n "$latest" ]]; then
                display_context "$latest"
            fi
            ;;
        "")
            # Interactive mode
            local selected
            selected=$(select_context "")
            if [[ $? -eq 0 && -n "$selected" ]]; then
                display_context "$selected"
            fi
            ;;
        *)
            # Filter mode
            local contexts
            contexts=$(list_contexts "$arg")
            local exit_code=$?
            
            if [[ $exit_code -ne 0 ]]; then
                return 1
            fi
            
            local filepaths=()
            while IFS= read -r line; do
                filepaths+=("$line")
            done <<< "$contexts"
            
            if [[ ${#filepaths[@]} -eq 0 ]]; then
                echo -e "${YELLOW}No contexts match filter: $arg${NC}" >&2
                return 1
            elif [[ ${#filepaths[@]} -eq 1 ]]; then
                display_context "${filepaths[0]}"
            else
                echo -e "${CYAN}Multiple contexts match. Select one:${NC}" >&2
                local selected
                selected=$(select_context "$arg")
                if [[ $? -eq 0 && -n "$selected" ]]; then
                    display_context "$selected"
                fi
            fi
            ;;
    esac
}

# Only run main when executed directly, not when sourced
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi