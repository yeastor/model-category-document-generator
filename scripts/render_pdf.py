import json
import sys
from pathlib import Path

from reportlab.lib.pagesizes import A4
from reportlab.lib.styles import ParagraphStyle, getSampleStyleSheet
from reportlab.lib.units import mm
from reportlab.pdfbase import pdfmetrics
from reportlab.pdfbase.ttfonts import TTFont
from reportlab.platypus import PageBreak, Paragraph, SimpleDocTemplate, Spacer


def main() -> int:
    payload_path = Path(sys.argv[1])
    output_path = Path(sys.argv[2])
    payload = json.loads(payload_path.read_text(encoding="utf-8"))

    font_path = Path("C:/Windows/Fonts/arial.ttf")
    font_name = "ArialCustom"
    pdfmetrics.registerFont(TTFont(font_name, str(font_path)))

    styles = getSampleStyleSheet()
    normal = ParagraphStyle(
        "NormalRu",
        parent=styles["Normal"],
        fontName=font_name,
        fontSize=12,
        leading=17,
        spaceAfter=7,
    )
    title = ParagraphStyle(
        "TitleRu",
        parent=normal,
        fontSize=15,
        leading=20,
        alignment=1,
        spaceBefore=10,
        spaceAfter=16,
    )

    document = SimpleDocTemplate(
        str(output_path),
        pagesize=A4,
        rightMargin=18 * mm,
        leftMargin=18 * mm,
        topMargin=22 * mm,
        bottomMargin=22 * mm,
    )

    story = []
    for line in payload["text"].splitlines():
        stripped = line.strip()
        if not stripped:
            story.append(Spacer(1, 8))
            continue
        if stripped == "Приложение № 1":
            story.append(PageBreak())
        style = title if stripped == payload["title"] else normal
        if stripped == "Приложение № 1":
            style = title
        story.append(Paragraph(stripped, style))

    story.append(Spacer(1, 14))
    story.append(Paragraph("Документ создан прототипом генератора.", normal))
    document.build(story)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
