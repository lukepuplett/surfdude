package surf

import (
	"context"
	"fmt"
	"time"
)

// Wave is a journey written as a chain, in the style of the C# client:
//
//	report, err := surf.NewWave(s).
//		FromRoot().
//		Then("search-listings", surf.With(surf.Values{"make": "MINI"})).
//		ThenItem(0).
//		ThenSubmit("shortlist", surf.Values{"note": "nice colour"}).
//		Go(ctx)
//
// Nothing happens until Go. The first step that fails stops the wave, and the report shows how far
// it got.
type Wave struct {
	s     *Surfer
	steps []waveStep
}

type waveStep struct {
	name string
	run  func(ctx context.Context, st *waveState) error
}

type waveState struct {
	page *Page
	item *ItemPage
}

// ReportLine is one step that ran.
type ReportLine struct {
	Step     string
	URL      string
	Status   int
	Duration time.Duration
}

// Report says how a wave went.
type Report struct {
	Lines []ReportLine
}

// NewWave starts a journey.
func NewWave(s *Surfer) *Wave { return &Wave{s: s} }

// FromRoot opens the front door.
func (w *Wave) FromRoot() *Wave {
	return w.add("from root", func(ctx context.Context, st *waveState) error {
		p, err := w.s.Root(ctx)
		st.page, st.item = p, nil
		return err
	})
}

// Then follows a link, or a form that needs no values, on the current page.
func (w *Wave) Then(rel string, opts ...Step) *Wave {
	return w.add("then "+rel, func(ctx context.Context, st *waveState) error {
		return st.move(ctx, rel, nil, opts)
	})
}

// ThenSubmit submits a form on the current page or item.
func (w *Wave) ThenSubmit(rel string, v Values, opts ...Step) *Wave {
	return w.add("then submit "+rel, func(ctx context.Context, st *waveState) error {
		return st.move(ctx, rel, v, opts)
	})
}

// ThenItem selects a member of the current list, so the next step uses its controls.
func (w *Wave) ThenItem(i int) *Wave {
	return w.add(fmt.Sprintf("then item %d", i), func(ctx context.Context, st *waveState) error {
		if st.page == nil {
			return fmt.Errorf("Unable to select item %d. The wave has no page yet.", i)
		}
		it, err := st.page.Item(i)
		st.item = it
		return err
	})
}

// ThenRead decodes the current item, or the current page's first item, into a value.
func (w *Wave) ThenRead(into any) *Wave {
	return w.add("then read", func(ctx context.Context, st *waveState) error {
		if st.item != nil {
			return st.item.Read(into)
		}
		return st.page.Read(into)
	})
}

func (w *Wave) add(name string, run func(context.Context, *waveState) error) *Wave {
	w.steps = append(w.steps, waveStep{name: name, run: run})
	return w
}

func (st *waveState) move(ctx context.Context, rel string, v Values, opts []Step) error {
	var next *Page
	var err error
	switch {
	case st.item != nil && v != nil:
		next, err = st.item.Submit(ctx, rel, v, opts...)
	case st.item != nil:
		next, err = st.item.Follow(ctx, rel, opts...)
	case v != nil:
		next, err = st.page.Submit(ctx, rel, v, opts...)
	default:
		next, err = st.page.Follow(ctx, rel, opts...)
	}
	if next != nil && err == nil {
		st.page, st.item = next, nil
	}
	return err
}

// Go runs the wave.
func (w *Wave) Go(ctx context.Context) (*Report, error) {
	report := &Report{}
	st := &waveState{}
	for _, step := range w.steps {
		start := time.Now()
		err := step.run(ctx, st)
		line := ReportLine{Step: step.name, Duration: time.Since(start)}
		if st.page != nil {
			line.URL, line.Status = st.page.URL.String(), st.page.Status
		}
		report.Lines = append(report.Lines, line)
		if err != nil {
			return report, fmt.Errorf("The wave stopped at %q: %w", step.name, err)
		}
	}
	return report, nil
}
