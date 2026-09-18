import React from "react";
import Link from "@docusaurus/Link";

import "./styles.css";

const GuideCard = ({ href, title, description }) => (
  <Link className="guide-card-href" href={href}>
    <div className="guide-card-container">
      {/* Ordinal comes from a CSS counter, so it stays right if cards are reordered. */}
      <div className="guide-card-ordinal" aria-hidden="true" />
      <div className="guide-card-detail">
        <div className="guide-card-title">{title}</div>
        <div className="guide-card-description">{description}</div>
      </div>
    </div>
  </Link>
);

export default GuideCard;
