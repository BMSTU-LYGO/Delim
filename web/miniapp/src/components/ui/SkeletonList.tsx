interface SkeletonListProps {
  count?: number;
}

export function SkeletonList({ count = 4 }: SkeletonListProps) {
  return (
    <div aria-label="Загрузка списка" className="skeleton-list" role="status">
      {Array.from({ length: count }, (_, index) => (
        <div className="skeleton-list__row" key={index}>
          <span className="skeleton skeleton--avatar" />
          <span className="skeleton-list__text">
            <span className="skeleton skeleton--title" />
            <span className="skeleton skeleton--subtitle" />
          </span>
        </div>
      ))}
    </div>
  );
}
