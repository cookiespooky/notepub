package mediautil

import "testing"

func TestFinderResolvesLikeObsidian(t *testing.T) {
	f := NewFinder([]string{"foto/komod.svg", "raboty/local.png", "logo.svg", "a/b/dup.jpg", "c/dup.jpg", "Фото/Диван 1.jpg"})
	cases := []struct{ target, base, want string }{
		{"komod.svg", "raboty/komod-1950.md", "foto/komod.svg"}, // по имени из другой папки
		{"local.png", "raboty/x.md", "raboty/local.png"},        // рядом с заметкой
		{"logo.svg", "deep/dir/x.md", "logo.svg"},               // от корня
		{"foto/komod.svg", "x.md", "foto/komod.svg"},            // путь от корня
		{"dup.jpg", "x.md", "c/dup.jpg"},                        // короткий путь побеждает
		{"Диван 1.jpg", "notes/x.md", "Фото/Диван 1.jpg"},       // кириллица и пробел
		{"%D0%94%D0%B8%D0%B2%D0%B0%D0%BD%201.jpg", "x.md", "Фото/Диван 1.jpg"},
		{"KOMOD.SVG", "x.md", "foto/komod.svg"}, // регистр не важен
	}
	for _, c := range cases {
		got, ok := f.Find(c.target, c.base, "")
		if !ok || got != c.want {
			t.Errorf("Find(%q, %q) = %q, %v; want %q", c.target, c.base, got, ok, c.want)
		}
	}
	if _, ok := f.Find("missing.png", "x.md", ""); ok {
		t.Error("несуществующий файл найден")
	}
	if _, ok := (*Finder)(nil).Find("komod.svg", "x.md", ""); ok {
		t.Error("пустой поиск ничего не должен находить")
	}
}
