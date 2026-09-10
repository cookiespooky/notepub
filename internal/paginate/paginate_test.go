package paginate

import "testing"

func TestPageCount(t *testing.T) {
	tests := []struct{ total, perPage, want int }{
		{0, 10, 1}, // пустая лента всё равно имеет первую страницу
		{1, 10, 1},
		{10, 10, 1}, // ровно на страницу — вторая не нужна
		{11, 10, 2},
		{20, 10, 2},
		{21, 10, 3},
		{14, 10, 2}, // текущий блог сайта
		{5, 0, 1},   // per_page не задан — не делим
	}
	for _, tt := range tests {
		if got := PageCount(tt.total, tt.perPage); got != tt.want {
			t.Errorf("PageCount(%d, %d) = %d, want %d", tt.total, tt.perPage, got, tt.want)
		}
	}
}

func TestBounds(t *testing.T) {
	tests := []struct {
		total, perPage, page int
		wantStart, wantEnd   int
	}{
		{14, 10, 1, 0, 10},
		{14, 10, 2, 10, 14}, // последняя страница короче
		{20, 10, 2, 10, 20},
		{14, 10, 3, 14, 14}, // страницы нет — пустой диапазон, не паника
		{14, 10, 0, 0, 14},
	}
	for _, tt := range tests {
		start, end := Bounds(tt.total, tt.perPage, tt.page)
		if start != tt.wantStart || end != tt.wantEnd {
			t.Errorf("Bounds(%d, %d, %d) = %d,%d want %d,%d",
				tt.total, tt.perPage, tt.page, start, end, tt.wantStart, tt.wantEnd)
		}
	}
}

// Каждая страница должна показывать все элементы ровно один раз: ни одна статья
// не теряется между страницами и ни одна не показывается дважды.
func TestBoundsCoverAllItemsOnce(t *testing.T) {
	const total, perPage = 14, 10
	seen := make([]int, total)
	for page := 1; page <= PageCount(total, perPage); page++ {
		start, end := Bounds(total, perPage, page)
		for i := start; i < end; i++ {
			seen[i]++
		}
	}
	for i, n := range seen {
		if n != 1 {
			t.Fatalf("элемент %d показан %d раз, ожидался ровно один", i, n)
		}
	}
}

func TestPath(t *testing.T) {
	tests := []struct {
		tmpl string
		n    int
		want string
	}{
		{"/blog/page/{{ n }}/", 2, "/blog/page/2/"},
		{"/blog/page/{{n}}/", 3, "/blog/page/3/"},
		{"blog/page/{{ n }}/", 2, "/blog/page/2/"},
		{"", 2, ""},
	}
	for _, tt := range tests {
		if got := Path(tt.tmpl, tt.n); got != tt.want {
			t.Errorf("Path(%q, %d) = %q, want %q", tt.tmpl, tt.n, got, tt.want)
		}
	}
}
