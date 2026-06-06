---
name: init-ai-workflow
description: 项目级技能：用于在一个全新的项目中初始化 AI 代理开发工作流质量体系，包括生成 AGENTS.md 以及 docs 目录下的设计、规范、参考等基础文档结构。
---

# Initialize AI Development Workflow Skill

当用户要求在当前（全新或现有）项目中“初始化 AI 开发质量流”、“建立 Agents.md”或“迁移 AI 开发方案”时，你**必须**执行本技能来构建完整的分层文档骨架。

## 目录与文件架构

本技能将在当前工作区根目录下创建以下文件和目录：
- `AGENTS.md` (入口指引文件)
- `docs/design/` (系统设计与架构)
- `docs/guideline/` (开发指导规范)
- `docs/reference/` (参考手册)
- `docs/deployment/` (部署指南)
- `docs/plan/` (开发计划与交接)
- `docs/changelog/` (变更日志)

*(注意: 默认不需要生成 guide 目录)*

## 执行工作流 (Workflow)

你可以通过执行本项目技能目录中自带的初始化脚本来一键生成所有骨架代码：

1. **运行初始化脚本**
   使用你的终端执行工具运行此脚本：`bash .agent/skills/init-ai-workflow/scripts/init.sh`
   *(或者如果在其他项目中迁移，你可以读取本脚本的内容并在目标项目中执行，或者直接复制整个技能目录过去执行)*

2. **验证生成结果**
   确认 `AGENTS.md` 已在根目录生成，并且 `docs/` 目录下的各子目录 (`design`, `guideline`, `reference`, `deployment`, `plan`, `changelog`) 及其骨架文件均已就绪。

3. **定制化 (如需)**
   根据当前项目的具体技术栈和需求，对 `docs/guideline/development-constraints.md` 等模板文件中的内容做初步定制或提醒用户自行补充。

## 资源文件说明

本技能依赖以下脚本完成核心工作：
- `scripts/init.sh`: 包含自动化创建目录和各类 Markdown 模板的 Bash 脚本。
