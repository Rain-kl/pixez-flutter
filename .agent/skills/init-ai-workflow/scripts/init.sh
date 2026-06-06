#!/usr/bin/env bash
# Initialize AI Development Workflow directory structure and template files

echo "Initializing AI Development Workflow..."

# 1. Create root AGENTS.md
cat << 'EOF' > AGENTS.md
# AGENTS.md

本文件是本项目 AI 代理接手的入口，不承载详细设计、规范和计划。接手项目时，请根据以下分层文档指引进行阅读与开发：

### 1. 开发指导规范 (AI & Developer Guidelines)

* **必须阅读**：
  * **[docs/guideline/development-constraints.md](docs/guideline/development-constraints.md)**：掌握核心的开发约束、数据模型规范、API 设计准则及变更准入标准。
  * **[docs/guideline/Role.md](docs/guideline/Role.md)**：通用的高质量编码准则，包括架构、并发、错误处理、安全及工作流程。
* **正在进行的开发计划与接手 (Handover & Plans)**：
  * **[docs/plan/index.md](docs/plan/index.md)**：查看正在进行的开发实现计划（Implementation Plan）与 AI 代理交接文档（Handover），接手项目时优先检查。

### 2. 系统设计与架构 (Design Docs)

* **[docs/design/index.md](docs/design/index.md)**：理解产品范围、系统边界、核心对象及长期约束。
* **[docs/design/architecture.md](docs/design/architecture.md)**：理解各模块的职责边界与拓扑架构。

### 3. 部署与参考手册 (Deployment & References)

* **[docs/deployment/deployment.md](docs/deployment/deployment.md)**：部署配置，接入、升级与维护策略。
* **[docs/reference/configuration.md](docs/reference/configuration.md)** / **[cli.md](docs/reference/cli.md)**：支持的环境变量、参数、命令行与配置文件参考。

---

## 开发与执行要求

1. **设计先行**：开发新功能或重要特性时，必须在 `docs/design/` 下创建或更新设计文档。
2. **遵守约束**：必须严格遵循 `docs/guideline/` 下的所有开发准则与约束。
3. **开发计划与交接**：正在进行的开发计划或 AI 接手交接发生变化时，在 `docs/plan/` 下更新对应的文档。
4. **文档与变更日志**：
   * 代码或配置变更完成后，必须在 [`docs/changelog/index.md`](docs/changelog/index.md) 的 `[Unreleased]` 区块补充对应变更条目。
   * 纯文档变更（如 `docs/` 下的 Markdown 文档、README 等）不需要写入 changelog。
EOF

# 2. Create directories
mkdir -p docs/design docs/guideline docs/reference docs/deployment docs/plan docs/changelog

# 3. Create placeholders
cat << 'EOF' > docs/guideline/development-constraints.md
# Development Constraints

请在此填写本项目的核心开发约束、数据模型规范、API 设计准则及变更准入标准。
EOF

cat << 'EOF' > docs/guideline/Role.md
# Role Guidelines

请在此填写本项目的通用高质量编码准则，如代码风格、架构原则、并发、错误处理、安全及工作流程。
EOF

cat << 'EOF' > docs/plan/index.md
# Plans & Handover

包含以下类型文档：
1. **Implementation Plan**：开发实现计划。
2. **Handover**：AI 代理交接文档。
EOF

cat << 'EOF' > docs/design/index.md
# System Design

本目录包含系统的详细设计文档。
EOF

cat << 'EOF' > docs/design/architecture.md
# Architecture

本项目的整体系统架构与模块拓扑结构设计。
EOF

cat << 'EOF' > docs/deployment/deployment.md
# Deployment Guide

本项目的部署和维护指南。
EOF

cat << 'EOF' > docs/reference/configuration.md
# Configuration Reference

本项目的配置项、环境变量参考手册。
EOF

cat << 'EOF' > docs/reference/cli.md
# CLI Reference

本项目的命令行工具参数说明。
EOF

cat << 'EOF' > docs/changelog/index.md
# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]
EOF

echo "AI Development Workflow initialized successfully!"
